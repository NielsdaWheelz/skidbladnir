import org.jetbrains.kotlin.gradle.dsl.JvmTarget
import java.util.Properties

plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
    id("org.jetbrains.kotlin.plugin.compose")
    id("org.jetbrains.kotlin.plugin.serialization")
}

private data class SigningMaterial(
    val storeFile: File,
    val password: String,
    val keyAlias: String,
)

private fun loadSigningMaterial(path: String): SigningMaterial {
    val configFile = File(path)
    val properties = Properties().apply { configFile.inputStream().use(::load) }
    val expectedKeys = setOf("storeFile", "passwordFile", "keyAlias")
    require(properties.stringPropertyNames() == expectedKeys) {
        "Android signing configuration must contain exactly storeFile, passwordFile, and keyAlias"
    }
    fun requiredAbsoluteFile(key: String): File {
        val value = requireNotNull(properties.getProperty(key)) { "Missing Android signing property: $key" }
        return File(value).also { require(it.isAbsolute) { "$key must be absolute" } }
    }
    val passwordLines = requiredAbsoluteFile("passwordFile").readLines()
    require(passwordLines.size == 1 && passwordLines.single().matches(Regex("[A-Za-z0-9]{32,128}"))) {
        "Android signing password file must contain one strong ASCII token"
    }
    return SigningMaterial(
        storeFile = requiredAbsoluteFile("storeFile"),
        password = passwordLines.single(),
        keyAlias = requireNotNull(properties.getProperty("keyAlias")),
    )
}

private val signingMaterial = providers.environmentVariable("SKIDBLADNIR_ANDROID_SIGNING_CONFIG")
    .orNull
    ?.let(::loadSigningMaterial)

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
        minSdk = 36
        targetSdk = 36
        versionCode = 1
        versionName = "0.1.0"
        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"
    }

    buildFeatures {
        compose = true
    }

    val skidbladnirSigning = signingMaterial?.let { material ->
        signingConfigs.create("skidbladnir") {
            storeFile = material.storeFile
            storePassword = material.password
            keyAlias = material.keyAlias
            keyPassword = material.password
            storeType = "PKCS12"
        }
    }

    buildTypes {
        release {
            isMinifyEnabled = false
            signingConfig = skidbladnirSigning
        }
        if (skidbladnirSigning != null) {
            create("deviceDebug") {
                initWith(getByName("debug"))
                signingConfig = skidbladnirSigning
                matchingFallbacks += listOf("debug")
            }
        }
    }

    if (skidbladnirSigning != null) {
        sourceSets.getByName("deviceDebug") {
            java.srcDir("src/debug/java")
            manifest.srcFile("src/debug/AndroidManifest.xml")
        }
    }

    testBuildType = providers.gradleProperty("skidbladnir.android.testBuildType")
        .getOrElse("debug")

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    testOptions {
        animationsDisabled = true
    }

    sourceSets.getByName("debug") {
        // The debug-only seal gallery (48dp acceptance instrument) reads the
        // canonical catalogue straight from the repo — no copied snapshot.
        assets.srcDir(rootProject.file("../catalog"))
    }

}

gradle.taskGraph.whenReady {
    val requestsProtectedArtifact = allTasks.any { task ->
        task.project == project && (task.name.contains("Release") || task.name.contains("DeviceDebug"))
    }
    if (requestsProtectedArtifact && signingMaterial == null) {
        throw GradleException("Protected Android artifacts require SKIDBLADNIR_ANDROID_SIGNING_CONFIG")
    }
}

if (signingMaterial != null) {
    configurations.named("deviceDebugImplementation") {
        extendsFrom(configurations.getByName("debugImplementation"))
    }
}

dependencies {
    implementation(platform("androidx.compose:compose-bom:2026.06.00"))
    implementation("androidx.activity:activity-compose:1.13.0")
    implementation("androidx.compose.foundation:foundation")
    implementation("androidx.compose.material3:material3")
    implementation("androidx.compose.ui:ui")
    implementation("androidx.compose.ui:ui-tooling-preview")
    implementation("androidx.webkit:webkit:1.17.0")
    implementation("org.jetbrains.kotlinx:kotlinx-serialization-json:1.7.3")
    implementation("com.squareup.okhttp3:okhttp:4.12.0")

    testImplementation("junit:junit:4.13.2")

    androidTestImplementation(platform("androidx.compose:compose-bom:2026.06.00"))
    androidTestImplementation("androidx.compose.ui:ui-test-junit4")
    androidTestImplementation("androidx.test:core-ktx:1.7.0")
    androidTestImplementation("androidx.test:runner:1.7.0")
    androidTestImplementation("androidx.test:rules:1.7.0")
    androidTestImplementation("androidx.test.ext:junit:1.3.0")
    androidTestImplementation("androidx.compose.ui:ui-test-junit4")
    debugImplementation("androidx.compose.ui:ui-test-manifest")
}
