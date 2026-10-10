//go:build darwin && cgo

#import <AppKit/AppKit.h>
#import <CoreServices/CoreServices.h>
#import <UserNotifications/UserNotifications.h>
#include <math.h>
#include <unistd.h>
#import "native.h"

extern void goSkidNativeEvent(char *encoded);

static NSURL *ApplicationURL(const char *application_path) {
    NSString *path = [NSString stringWithUTF8String:application_path];
    if (!path.isAbsolutePath || ![path.pathExtension isEqual:@"app"]) return nil;
    // Keep the installer's logical path. A physical generation is not a substitute.
    NSURL *url = [NSURL fileURLWithPath:path isDirectory:YES].URLByStandardizingPath;
    NSBundle *bundle = [NSBundle bundleWithURL:url];
    if (![bundle.bundleIdentifier isEqual:@"dev.niels.skidbladnir"] ||
        ![bundle.infoDictionary[@"CFBundlePackageType"] isEqual:@"APPL"] ||
        ![bundle.infoDictionary[@"CFBundleExecutable"] isEqual:@"skid-notifications"]) return nil;
    return url;
}

static NSArray<NSRunningApplication *> *ApplicationsAtURL(NSURL *url) {
    NSMutableArray<NSRunningApplication *> *matches = [NSMutableArray new];
    NSURL *executable = [url URLByAppendingPathComponent:@"Contents/MacOS/skid-notifications"];
    for (NSRunningApplication *app in NSWorkspace.sharedWorkspace.runningApplications) {
        if (!app.isTerminated && app.processIdentifier != getpid() &&
            [app.bundleIdentifier isEqual:@"dev.niels.skidbladnir"] &&
            [app.bundleURL.URLByStandardizingPath isEqual:url] &&
            [app.executableURL.URLByStandardizingPath isEqual:executable]) [matches addObject:app];
    }
    return matches;
}

int32_t skid_native_register(const char *application_path) {
    @autoreleasepool {
        NSURL *url = ApplicationURL(application_path);
        if (url == nil) return paramErr;
        return LSRegisterURL((__bridge CFURLRef)url, true);
    }
}

int32_t skid_native_status(const char *application_path) {
    @autoreleasepool {
        NSURL *url = ApplicationURL(application_path);
        if (url == nil) return 0;
        NSArray<NSRunningApplication *> *apps = ApplicationsAtURL(url);
        if (apps.count != 1 || !apps[0].isFinishedLaunching || apps[0].processIdentifier <= 0) return 0;
        return apps[0].processIdentifier;
    }
}

int32_t skid_native_stop_application(const char *application_path) {
    @autoreleasepool {
        NSTimeInterval deadline = NSProcessInfo.processInfo.systemUptime + 3.0;
        NSURL *url = ApplicationURL(application_path);
        if (url == nil) return 1;
        NSArray<NSRunningApplication *> *apps = ApplicationsAtURL(url);
        for (NSRunningApplication *app in apps) {
            if (NSProcessInfo.processInfo.systemUptime >= deadline || ![app terminate]) return 1;
        }
        while (NSProcessInfo.processInfo.systemUptime < deadline) {
            BOOL exited = YES;
            for (NSRunningApplication *app in apps) if (!app.isTerminated) exited = NO;
            if (exited && ApplicationsAtURL(url).count == 0) return 0;
            NSTimeInterval remaining = deadline - NSProcessInfo.processInfo.systemUptime;
            if (remaining <= 0) return 1;
            // Cocoa refreshes application state only when its main run loop advances.
            if (![NSRunLoop.currentRunLoop runMode:NSDefaultRunLoopMode beforeDate:
                  [NSDate dateWithTimeIntervalSinceNow:fmin(0.02, remaining)]]) return 1;
        }
        return 1;
    }
}

static void Emit(NSDictionary *event) {
    NSData *data = [NSJSONSerialization dataWithJSONObject:event options:0 error:nil];
    if (data != nil) {
        NSString *encoded = [[NSString alloc] initWithData:data encoding:NSUTF8StringEncoding];
        goSkidNativeEvent((char *)encoded.UTF8String);
    }
}

static void Reply(uint64_t request, BOOL ok, id value) {
    Emit(@{@"kind": @"result", @"request": @(request), @"ok": @(ok), @"value": value ?: @{}});
}

static NSDictionary *Notice(UNNotificationRequest *request) {
    NSDictionary *metadata = request.content.userInfo;
    if (![metadata[@"episode"] isKindOfClass:NSNumber.class] ||
        ![metadata[@"readyGeneration"] isKindOfClass:NSNumber.class] ||
        ![metadata[@"epoch"] isKindOfClass:NSString.class] ||
        ![metadata[@"ref"] isKindOfClass:NSString.class] ||
        ![metadata[@"slot"] isEqual:request.identifier]) return nil;
    return @{@"slot": request.identifier, @"episode": metadata[@"episode"],
             @"epoch": metadata[@"epoch"], @"readyGeneration": metadata[@"readyGeneration"],
             @"ref": metadata[@"ref"], @"title": request.content.title,
             @"body": request.content.body};
}

static NSAppleEventDescriptor *Script(NSString *source) {
    NSAppleScript *script = [[NSAppleScript alloc] initWithSource:source];
    NSDictionary *error = nil;
    NSAppleEventDescriptor *result = [script executeAndReturnError:&error];
    return error == nil ? result : nil;
}

@interface SkidDelegate : NSObject <NSApplicationDelegate, UNUserNotificationCenterDelegate>
@property(nonatomic) NSWindow *setupWindow;
@property(nonatomic) NSTextField *health;
- (void)showWindow;
@end

@implementation SkidDelegate
- (void)applicationDidFinishLaunching:(NSNotification *)notification {
    UNUserNotificationCenter *center = UNUserNotificationCenter.currentNotificationCenter;
    UNNotificationCategory *category = [UNNotificationCategory categoryWithIdentifier:@"needs-input"
        actions:@[] intentIdentifiers:@[] options:UNNotificationCategoryOptionCustomDismissAction];
    [center setNotificationCategories:[NSSet setWithObject:category]];
    [NSWorkspace.sharedWorkspace.notificationCenter addObserverForName:NSWorkspaceDidWakeNotification
        object:nil queue:NSOperationQueue.mainQueue usingBlock:^(NSNotification *notice) {
            Emit(@{@"kind": @"wake"});
        }];
    Emit(@{@"kind": @"ready"});
}

- (BOOL)applicationShouldHandleReopen:(NSApplication *)application hasVisibleWindows:(BOOL)visible {
    return NO;
}

- (BOOL)applicationShouldTerminateAfterLastWindowClosed:(NSApplication *)application { return NO; }

- (NSApplicationTerminateReply)applicationShouldTerminate:(NSApplication *)application {
    Emit(@{@"kind": @"quit"});
    return NSTerminateCancel;
}

- (void)applicationWillTerminate:(NSNotification *)notification { Emit(@{@"kind": @"quit"}); }

- (void)userNotificationCenter:(UNUserNotificationCenter *)center
      willPresentNotification:(UNNotification *)notification
        withCompletionHandler:(void (^)(UNNotificationPresentationOptions))completion {
    if (notification.request.content.interruptionLevel == UNNotificationInterruptionLevelPassive) {
        completion(UNNotificationPresentationOptionList);
    } else {
        completion(UNNotificationPresentationOptionBanner | UNNotificationPresentationOptionList |
                   UNNotificationPresentationOptionSound);
    }
}

- (void)userNotificationCenter:(UNUserNotificationCenter *)center
       didReceiveNotificationResponse:(UNNotificationResponse *)response
                withCompletionHandler:(void (^)(void))completion {
    NSDictionary *notice = Notice(response.notification.request);
    if (notice != nil) {
        NSString *kind = [response.actionIdentifier isEqual:UNNotificationDismissActionIdentifier] ?
            @"dismiss" : @"click";
        Emit(@{@"kind": kind, @"slot": notice[@"slot"], @"episode": notice[@"episode"],
               @"epoch": notice[@"epoch"], @"ref": notice[@"ref"]});
    }
    completion();
}

- (void)showWindow {
    if (self.setupWindow == nil) {
        NSRect frame = NSMakeRect(0, 0, 450, 235);
        self.setupWindow = [[NSWindow alloc] initWithContentRect:frame
            styleMask:NSWindowStyleMaskTitled | NSWindowStyleMaskClosable
            backing:NSBackingStoreBuffered defer:NO];
        self.setupWindow.title = @"skid notifications";
        self.setupWindow.releasedWhenClosed = NO;
        [self.setupWindow center];
        NSView *content = self.setupWindow.contentView;
        self.health = [NSTextField wrappingLabelWithString:@"notification setup required."];
        self.health.frame = NSMakeRect(24, 160, 402, 50);
        [content addSubview:self.health];
        NSTextField *instructions = [NSTextField wrappingLabelWithString:
            @"keep tailscale connected. notification setup uses the paired devbox in skid’s client configuration."];
        instructions.frame = NSMakeRect(24, 110, 402, 48);
        [content addSubview:instructions];
        NSArray<NSString *> *titles = @[@"allow notifications", @"open notification settings", @"reset notification memory"];
        SEL actions[] = {@selector(allow:), @selector(settings:), @selector(reset:)};
        for (NSUInteger index = 0; index < titles.count; index++) {
            NSButton *button = [NSButton buttonWithTitle:titles[index] target:self action:actions[index]];
            button.frame = NSMakeRect(24, 80 - index * 29, 270, 26);
            [content addSubview:button];
        }
    }
    [self.setupWindow makeKeyAndOrderFront:nil];
    [NSApp activateIgnoringOtherApps:YES];
    Emit(@{@"kind": @"setup"});
}

- (void)allow:(id)sender {
    [UNUserNotificationCenter.currentNotificationCenter
        requestAuthorizationWithOptions:UNAuthorizationOptionAlert | UNAuthorizationOptionSound
        completionHandler:^(BOOL allowed, NSError *error) {
            Emit(@{@"kind": @"permission", @"value": @{@"allowed": @(allowed),
                @"errorDomain": error.domain ?: @"", @"errorCode": @(error.code)}});
        }];
}

- (void)settings:(id)sender {
    [NSWorkspace.sharedWorkspace openURL:[NSURL URLWithString:
        @"x-apple.systempreferences:com.apple.Notifications-Settings.extension?id=dev.niels.skidbladnir"]];
}

- (void)reset:(id)sender { Emit(@{@"kind": @"reset"}); }
@end

static SkidDelegate *delegate;

void skid_native_run(void) {
    @autoreleasepool {
        [NSApplication sharedApplication];
        [NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
        delegate = [SkidDelegate new];
        NSApp.delegate = delegate;
        UNUserNotificationCenter.currentNotificationCenter.delegate = delegate;
        [NSApp run];
    }
}

void skid_native_stop(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        [NSApp stop:nil];
        NSEvent *event = [NSEvent otherEventWithType:NSEventTypeApplicationDefined
            location:NSZeroPoint modifierFlags:0 timestamp:0 windowNumber:0 context:nil
            subtype:0 data1:0 data2:0];
        [NSApp postEvent:event atStart:NO];
    });
}

void skid_native_send(uint64_t request, const char *operation, const char *payload) {
    @autoreleasepool {
        NSString *op = [NSString stringWithUTF8String:operation];
        NSData *data = [[NSString stringWithUTF8String:payload] dataUsingEncoding:NSUTF8StringEncoding];
        if ([op isEqual:@"post"]) {
            // UserNotifications permits initiation from any app thread. The Go
            // owner invokes this before accepting another hint; it never waits for
            // completion. AppKit and AppleEvents below remain on main.
            NSDictionary *args = [NSJSONSerialization JSONObjectWithData:data options:0 error:nil];
            UNMutableNotificationContent *content = [UNMutableNotificationContent new];
            content.title = args[@"title"];
            content.body = args[@"body"];
            content.categoryIdentifier = @"needs-input";
            content.userInfo = @{@"slot": args[@"slot"], @"epoch": args[@"epoch"], @"episode": args[@"episode"], @"ref": args[@"ref"], @"readyGeneration": args[@"readyGeneration"]};
            BOOL silent = [args[@"silent"] boolValue];
            content.sound = silent ? nil : UNNotificationSound.defaultSound;
            content.interruptionLevel = silent ? UNNotificationInterruptionLevelPassive : UNNotificationInterruptionLevelActive;
            UNNotificationRequest *notice = [UNNotificationRequest requestWithIdentifier:args[@"slot"] content:content trigger:nil];
            [UNUserNotificationCenter.currentNotificationCenter addNotificationRequest:notice withCompletionHandler:^(NSError *error) {
                Reply(request, error == nil, @{});
            }];
            return;
        }
        dispatch_async(dispatch_get_main_queue(), ^{
            NSDictionary *args = [NSJSONSerialization JSONObjectWithData:data options:0 error:nil];
            if ([op isEqual:@"setup"]) {
                [delegate showWindow];
                Reply(request, YES, @{});
                return;
            }
            UNUserNotificationCenter *center = UNUserNotificationCenter.currentNotificationCenter;
            if ([op isEqual:@"permission"]) {
                [center getNotificationSettingsWithCompletionHandler:^(UNNotificationSettings *settings) {
                    BOOL allowed = settings.authorizationStatus == UNAuthorizationStatusAuthorized ||
                        settings.authorizationStatus == UNAuthorizationStatusProvisional;
                    BOOL undetermined = settings.authorizationStatus == UNAuthorizationStatusNotDetermined;
                    Reply(request, YES, @{@"allowed": @(allowed),
                        @"undetermined": @(undetermined)});
                }];
            } else if ([op isEqual:@"inspect"]) {
                [center getPendingNotificationRequestsWithCompletionHandler:^(NSArray<UNNotificationRequest *> *pending) {
                    [center getDeliveredNotificationsWithCompletionHandler:^(NSArray<UNNotification *> *delivered) {
                        NSMutableArray *notices = [NSMutableArray new];
                        for (UNNotificationRequest *item in pending) {
                            NSDictionary *notice = Notice(item);
                            if (notice != nil) [notices addObject:notice];
                        }
                        for (UNNotification *item in delivered) {
                            NSDictionary *notice = Notice(item.request);
                            if (notice != nil) [notices addObject:notice];
                        }
                        Reply(request, YES, notices);
                    }];
                }];
            } else if ([op isEqual:@"cancel"]) {
                NSArray *identifiers = args[@"slots"];
                [center removePendingNotificationRequestsWithIdentifiers:identifiers];
                [center removeDeliveredNotificationsWithIdentifiers:identifiers];
                Reply(request, YES, @{});
            } else if ([op isEqual:@"focus"]) {
                if (![NSWorkspace.sharedWorkspace.frontmostApplication.bundleIdentifier isEqual:@"com.mitchellh.ghostty"]) {
                    Reply(request, YES, @{@"id": @""});
                } else {
                    NSAppleEventDescriptor *result = Script(@"with timeout of 2 seconds\n"
                        "tell application id \"com.mitchellh.ghostty\"\n"
                        "if frontmost then return id of focused terminal of selected tab of front window\n"
                        "return \"\"\nend tell\nend timeout");
                    Reply(request, result != nil, @{@"id": result.stringValue ?: @""});
                }
            } else if ([op isEqual:@"launch"]) {
                NSAppleEventDescriptor *result = Script(args[@"script"]);
                Reply(request, result != nil && result.stringValue.length > 0, @{@"id": result.stringValue ?: @""});
            } else if ([op isEqual:@"health"]) {
                if (delegate.health != nil) delegate.health.stringValue = args[@"text"];
                Reply(request, YES, @{});
            } else if ([op isEqual:@"error"]) {
                NSAlert *alert = [NSAlert new];
                alert.messageText = args[@"text"];
                [alert addButtonWithTitle:@"ok"];
                [NSApp activateIgnoringOtherApps:YES];
                [alert runModal];
                Reply(request, YES, @{});
            } else {
                Reply(request, NO, @{});
            }
        });
    }
}
