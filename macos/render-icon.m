// Build-time reuse of the existing android launcher vectors. The current
// ornament uses closed M/L polygons; a new command must extend this renderer
// explicitly rather than silently changing the mac identity.
#import <AppKit/AppKit.h>

static int Fail(const char *reason) {
    fprintf(stderr, "mac icon: %s\n", reason);
    return 1;
}

@interface Launcher : NSObject <NSXMLParserDelegate>
@property(nonatomic) NSMutableArray<NSDictionary *> *paths;
@property(nonatomic) BOOL valid;
@end

@implementation Launcher
- (instancetype)init {
    self = [super init];
    if (self != nil) { self.paths = [NSMutableArray new]; self.valid = YES; }
    return self;
}
- (void)parser:(NSXMLParser *)parser didStartElement:(NSString *)element namespaceURI:(NSString *)uri
 qualifiedName:(NSString *)qualified attributes:(NSDictionary<NSString *, NSString *> *)attributes {
    if ([element isEqual:@"vector"]) {
        if (![attributes[@"android:viewportWidth"] isEqual:@"108"] ||
            ![attributes[@"android:viewportHeight"] isEqual:@"108"]) self.valid = NO;
    } else if ([element isEqual:@"path"]) {
        if (attributes[@"android:fillColor"] == nil || attributes[@"android:pathData"] == nil) {
            self.valid = NO;
        } else {
            [self.paths addObject:@{@"color": attributes[@"android:fillColor"], @"path": attributes[@"android:pathData"]}];
        }
    } else self.valid = NO;
}
@end

int main(int argc, const char *argv[]) {
    @autoreleasepool {
        if (argc != 4) return 64;
        Launcher *launcher = [Launcher new];
        for (int index = 1; index <= 2; index++) {
            NSData *data = [NSData dataWithContentsOfFile:[NSString stringWithUTF8String:argv[index]]];
            if (data == nil) return Fail("launcher asset unavailable");
            NSXMLParser *parser = [[NSXMLParser alloc] initWithData:data];
            parser.delegate = launcher;
            if (![parser parse] || !launcher.valid) return Fail("launcher vector invalid");
        }
        NSString *output = [NSString stringWithUTF8String:argv[3]];
        NSDictionary<NSString *, NSNumber *> *sizes = @{
            @"icon_16x16.png": @16, @"icon_16x16@2x.png": @32,
            @"icon_32x32.png": @32, @"icon_32x32@2x.png": @64,
            @"icon_128x128.png": @128, @"icon_128x128@2x.png": @256,
            @"icon_256x256.png": @256, @"icon_256x256@2x.png": @512,
            @"icon_512x512.png": @512, @"icon_512x512@2x.png": @1024};
        for (NSString *name in sizes) {
            NSInteger size = sizes[name].integerValue;
            NSBitmapImageRep *bitmap = [[NSBitmapImageRep alloc]
                initWithBitmapDataPlanes:NULL pixelsWide:size pixelsHigh:size bitsPerSample:8
                samplesPerPixel:4 hasAlpha:YES isPlanar:NO colorSpaceName:NSDeviceRGBColorSpace
                bitmapFormat:0 bytesPerRow:0 bitsPerPixel:0];
            NSGraphicsContext *context = [NSGraphicsContext graphicsContextWithBitmapImageRep:bitmap];
            [NSGraphicsContext saveGraphicsState];
            NSGraphicsContext.currentContext = context;
            NSAffineTransform *transform = [NSAffineTransform transform];
            [transform translateXBy:0 yBy:size];
            [transform scaleXBy:size / 108.0 yBy:-size / 108.0];
            [transform concat];
            for (NSDictionary *item in launcher.paths) {
                NSString *color = item[@"color"];
                unsigned value = 0;
                NSScanner *hex = [NSScanner scannerWithString:color];
                if (color.length != 7 || ![hex scanString:@"#" intoString:NULL] || ![hex scanHexInt:&value] || !hex.atEnd) return Fail("launcher color invalid");
                [[NSColor colorWithSRGBRed:((value >> 16) & 255) / 255.0
                    green:((value >> 8) & 255) / 255.0 blue:(value & 255) / 255.0 alpha:1] setFill];
                NSScanner *scanner = [NSScanner scannerWithString:item[@"path"]];
                scanner.charactersToBeSkipped = [NSCharacterSet characterSetWithCharactersInString:@", "];
                NSBezierPath *path = [NSBezierPath bezierPath];
                while (!scanner.atEnd) {
                    if ([scanner scanString:@"Z" intoString:NULL]) {
                        [path closePath];
                    } else {
                        BOOL move = [scanner scanString:@"M" intoString:NULL];
                        if (!move && ![scanner scanString:@"L" intoString:NULL]) return Fail("launcher path command unsupported");
                        double x, y;
                        if (![scanner scanDouble:&x] || ![scanner scanDouble:&y]) return Fail("launcher path coordinate invalid");
                        if (move) [path moveToPoint:NSMakePoint(x, y)];
                        else [path lineToPoint:NSMakePoint(x, y)];
                    }
                }
                [path fill];
            }
            [NSGraphicsContext restoreGraphicsState];
            NSData *png = [bitmap representationUsingType:NSBitmapImageFileTypePNG properties:@{}];
            if (png == nil || ![png writeToFile:[output stringByAppendingPathComponent:name] atomically:YES]) return Fail("icon output unavailable");
        }
    }
    return 0;
}
