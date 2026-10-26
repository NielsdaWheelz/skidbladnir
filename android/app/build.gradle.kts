import groovy.json.JsonSlurper
import org.gradle.api.artifacts.component.ModuleComponentIdentifier
import org.jetbrains.kotlin.gradle.dsl.JvmTarget
import org.w3c.dom.Element
import java.util.Properties
import java.util.zip.ZipFile
import javax.xml.parsers.DocumentBuilderFactory

plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
    id("org.jetbrains.kotlin.plugin.compose")
    id("org.jetbrains.kotlin.plugin.serialization")
}

// scripts/release supplies these and scripts/check-release verifies the apk's signer and version; gradle only reads them.
private val signing = providers.environmentVariable("SKIDBLADNIR_ANDROID_SIGNING_CONFIG").orNull
    ?.let { path -> Properties().apply { File(path).inputStream().use(::load) } }
private val configuredVersionName = providers.gradleProperty("skidbladnir.versionName").orNull
private val configuredVersionCode = providers.gradleProperty("skidbladnir.versionCode").orNull
gradle.taskGraph.whenReady {
    if (allTasks.any { task -> task.project == project && task.name.contains("Release") }) {
        require(signing != null && configuredVersionName != null && configuredVersionCode != null) {
            "Release builds require SKIDBLADNIR_ANDROID_SIGNING_CONFIG, skidbladnir.versionName and skidbladnir.versionCode"
        }
    }
}

kotlin {
    compilerOptions {
        jvmTarget.set(JvmTarget.JVM_17)
    }
}

android {
    namespace = "dev.niels.skidbladnir"
    compileSdk = 36

    defaultConfig {
        applicationId = "dev.niels.skidbladnir"
        minSdk = 33
        targetSdk = 36
        versionCode = configuredVersionCode?.toInt() ?: 1
        versionName = configuredVersionName ?: "0.1.0"
    }

    buildFeatures {
        compose = true
    }

    val skidbladnirSigning = signing?.let { config ->
        val password = File(config.getProperty("passwordFile")).readLines().single()
        signingConfigs.create("skidbladnir") {
            storeFile = File(config.getProperty("storeFile"))
            storePassword = password
            keyAlias = config.getProperty("keyAlias")
            keyPassword = password
            storeType = "PKCS12"
        }
    }

    buildTypes {
        release {
            isMinifyEnabled = false
            signingConfig = skidbladnirSigning
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
}

// the bundled libraries' licenses, apache-2.0 among them, require their notices to ship with the apk, whose default
// packaging drops most license files. assets/licenses/third-party-notices.txt lists each runtime library with the
// licenses its pom or parent pom declares, then each distinct license or notice text the libraries carry, the
// third_party_licenses of google's libraries included.
abstract class ThirdPartyNotices : DefaultTask() {
    @get:OutputDirectory
    abstract val output: DirectoryProperty
}

fun pomLicenses(coordinates: String): List<String> {
    val pom = DocumentBuilderFactory.newInstance().newDocumentBuilder()
        .parse(configurations.detachedConfiguration(dependencies.create("$coordinates@pom")).singleFile)
    val licenses = pom.getElementsByTagName("license")
    if (licenses.length > 0) {
        return (0 until licenses.length).map { index ->
            val license = licenses.item(index) as Element
            listOf("name", "url").mapNotNull { license.getElementsByTagName(it).item(0)?.textContent?.trim() }.joinToString(", ")
        }
    }
    val parent = pom.getElementsByTagName("parent").item(0) as Element? ?: error("$coordinates declares no license")
    return pomLicenses(listOf("groupId", "artifactId", "version").joinToString(":") { parent.getElementsByTagName(it).item(0).textContent.trim() })
}

androidComponents {
    onVariants { variant ->
        val runtime = variant.runtimeConfiguration
        val notices = tasks.register<ThirdPartyNotices>("${variant.name}ThirdPartyNotices") {
            inputs.files(runtime)
            doLast {
                val texts = linkedMapOf<String, MutableList<String>>()
                fun carried(text: String, source: String) = texts.getOrPut(text) { mutableListOf() }.add(source)
                val licenseFile = Regex("(?i)(licen[cs]e|notice)(\\.txt)?")
                val libraries = runtime.incoming.artifacts.artifacts
                    .filter { it.id.componentIdentifier is ModuleComponentIdentifier }
                    .sortedBy { it.id.componentIdentifier.displayName }
                    .joinToString("\n") { library ->
                        val id = library.id.componentIdentifier.displayName
                        ZipFile(library.file).use { zip ->
                            for (entry in zip.entries()) {
                                if (licenseFile.matches(entry.name.substringAfterLast('/'))) {
                                    carried(zip.getInputStream(entry).readBytes().decodeToString(), "${entry.name} in $id")
                                }
                            }
                            zip.getEntry("third_party_licenses.json")?.let { index ->
                                val bundled = zip.getInputStream(zip.getEntry("third_party_licenses.txt")).readBytes()
                                @Suppress("UNCHECKED_CAST") // google's index maps each bundled library to its byte range.
                                (JsonSlurper().parse(zip.getInputStream(index)) as Map<String, Map<String, Int>>).forEach { (name, range) ->
                                    val start = range.getValue("start")
                                    carried(bundled.copyOfRange(start, start + range.getValue("length")).decodeToString(), "$name, bundled in $id")
                                }
                            }
                        }
                        "$id: ${pomLicenses(id).joinToString("; ")}"
                    }
                output.file("licenses/third-party-notices.txt").get().asFile.apply { parentFile.mkdirs() }.writeText(
                    "Third-party notices for the skid Android app. The fonts' licenses sit beside this file; xterm.js's are in terminal/vendor.\n\n" +
                        "The app bundles these libraries, each with the licenses its Maven POM declares:\n\n$libraries\n\n" +
                        "Each license or notice text the libraries carry follows once, after the places it appears.\n" +
                        texts.entries.joinToString("") { (text, sources) -> "\n== ${sources.joinToString("; ")}\n\n$text\n" },
                )
            }
        }
        variant.sources.assets?.addGeneratedSourceDirectory(notices, ThirdPartyNotices::output)
    }
}

dependencies {
    implementation(platform("androidx.compose:compose-bom:2026.06.00"))
    implementation("androidx.activity:activity-compose:1.13.0")
    implementation("androidx.fragment:fragment:1.9.1")
    implementation("androidx.compose.foundation:foundation")
    implementation("androidx.compose.material3:material3")
    implementation("androidx.compose.ui:ui")
    implementation("androidx.core:core:1.17.0")
    implementation("androidx.work:work-runtime-ktx:2.12.0")
    implementation("com.google.firebase:firebase-messaging:26.0.0")
    implementation("androidx.webkit:webkit:1.17.0")
    implementation("com.google.android.gms:play-services-code-scanner:16.1.0")
    implementation("org.jetbrains.kotlinx:kotlinx-serialization-json:1.7.3")
    implementation("com.squareup.okhttp3:okhttp:4.12.0")
}
