import org.jetbrains.kotlin.gradle.dsl.JvmTarget

plugins {
    // No 'org.jetbrains.kotlin.android': since AGP 9 Kotlin compilation is built into
    // the Android plugin (android.builtInKotlin), and the standalone plugin is not
    // compatible with the new DSL (android.newDsl). Compose/serialization/KSP stay.
    alias(libs.plugins.androidApplication)
    alias(libs.plugins.hilt)
    alias(libs.plugins.ksp)
    alias(libs.plugins.kotlinSerialization)
    alias(libs.plugins.kotlinCompose)
}

android {
    namespace = "com.commandercompanion"
    compileSdk = 37

    defaultConfig {
        applicationId = "com.commandercompanion"
        minSdk = 26
        targetSdk = 34
        versionCode = 1
        versionName = "1.0"

        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"
        vectorDrawables {
            useSupportLibrary = true
        }

        // Backend base URL. 10.0.2.2 is the alias the Android emulator uses to
        // reach "localhost" on the host machine (where `go run ./cmd/api` runs in dev).
        // Override without touching code: `./gradlew :app:assembleDebug -PAPI_BASE_URL=http://192.168.1.50:8080/`
        // (for a physical device on the same network) or by setting the property in gradle.properties.
        val apiBaseUrl = (project.findProperty("API_BASE_URL") as String?)
            ?: "http://10.0.2.2:8080/"
        buildConfigField("String", "API_BASE_URL", "\"$apiBaseUrl\"")

        // PLACEHOLDER: the real Web Client ID doesn't exist yet (see docs/roadmap/TASKS.md Stage 1,
        // "Create OAuth credentials in Google Cloud Console" — pending external manual step).
        // Until it's created, Credential Manager will fail the Google Sign-In flow with this
        // placeholder because Google doesn't recognize the client ID. Override: -PGOOGLE_WEB_CLIENT_ID=...
        val googleWebClientId = (project.findProperty("GOOGLE_WEB_CLIENT_ID") as String?)
            ?: "REPLACE_WITH_GOOGLE_CLOUD_WEB_CLIENT_ID.apps.googleusercontent.com"
        buildConfigField("String", "GOOGLE_WEB_CLIENT_ID", "\"$googleWebClientId\"")

        // Public URL of the web client. Only used to build the profile QR's
        // deep link (see QrEncoder/friendQrLink): the code has to encode the
        // same https://<web>/friends/add/{id} the web client encodes, or the
        // two clients could not scan each other. Not an API endpoint -- the
        // backend is reached through API_BASE_URL above.
        // Override: -PWEB_APP_URL=https://mi-despliegue.vercel.app
        val webAppUrl = (project.findProperty("WEB_APP_URL") as String?)
            ?: "http://10.0.2.2:3000"
        buildConfigField("String", "WEB_APP_URL", "\"$webAppUrl\"")
    }

    buildTypes {
        release {
            isMinifyEnabled = false
            proguardFiles(
                getDefaultProguardFile("proguard-android-optimize.txt"),
                "proguard-rules.pro"
            )
        }
    }
    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_11
        targetCompatibility = JavaVersion.VERSION_11
    }
    // Under built-in Kotlin this lives inside android { }, not in a top-level kotlin { } block.
    kotlin {
        compilerOptions {
            jvmTarget.set(JvmTarget.JVM_11)
        }
    }
    buildFeatures {
        compose = true
        buildConfig = true
    }
    packaging {
        resources {
            excludes += "/META-INF/{AL2.0,LGPL2.1}"
        }
    }
    testOptions {
        unitTests {
            // Unit tests touch types with loose framework dependencies
            // (e.g. android.util.Log via Room/Compose); without this they throw RuntimeException
            // "not mocked" instead of returning the type's default.
            isReturnDefaultValues = true
        }
    }
}

dependencies {
    implementation(libs.androidx.core.ktx)
    // AppCompatDelegate.setApplicationLocales(): per-app language override (Settings screen),
    // works with a plain ComponentActivity since AppCompat 1.6.0 — no need for AppCompatActivity.
    implementation(libs.androidx.appcompat)
    implementation(libs.androidx.lifecycle.runtime.ktx)
    implementation(libs.androidx.activity.compose)
    implementation(platform(libs.androidx.compose.bom))
    implementation(libs.androidx.ui)
    implementation(libs.androidx.ui.graphics)
    implementation(libs.androidx.ui.tooling.preview)
    implementation(libs.androidx.material3)

    // Navigation
    implementation(libs.navigation.compose)

    // Hilt
    implementation(libs.hilt.android)
    ksp(libs.hilt.compiler)
    implementation(libs.androidx.hilt.navigation.compose)
    implementation(libs.androidx.hilt.lifecycle.viewmodel.compose)

    // Room
    implementation(libs.room.runtime)
    implementation(libs.room.ktx)
    ksp(libs.room.compiler)

    // Retrofit & Serialization
    implementation(libs.retrofit)
    implementation(libs.retrofit.serialization)
    implementation(libs.kotlinx.serialization.json)

    // OkHttp (Retrofit's underlying HTTP client): auth + logging interceptor
    implementation(libs.okhttp)
    implementation(libs.okhttp.logging.interceptor)

    // DataStore: session persistence (access/refresh token + expiry)
    implementation(libs.androidx.datastore.preferences)

    // Credential Manager + Google Identity Services: real Google Sign-In
    implementation(libs.androidx.credentials)
    implementation(libs.androidx.credentials.play.services.auth)
    implementation(libs.googleid)

    // Coil: deck art thumbnails (commander card art from the backend's image_url)
    implementation(libs.zxing.core)
    implementation(libs.play.services.code.scanner)
    implementation(libs.coil.compose)
    implementation(libs.coil.network.okhttp)

    testImplementation(libs.junit)
    // runTest + Dispatchers.setMain to test ViewModels and repositories with coroutines.
    testImplementation(libs.kotlinx.coroutines.test)
    androidTestImplementation(libs.androidx.junit)
    androidTestImplementation(libs.androidx.espresso.core)
    androidTestImplementation(platform(libs.androidx.compose.bom))
    androidTestImplementation(libs.androidx.ui.test.junit4)
    debugImplementation(libs.androidx.ui.tooling)
    debugImplementation(libs.androidx.ui.test.manifest)
}
