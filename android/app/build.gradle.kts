import org.jetbrains.kotlin.gradle.dsl.JvmTarget
import java.util.Properties

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
