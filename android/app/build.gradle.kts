plugins {
    alias(libs.plugins.androidApplication)
    alias(libs.plugins.jetbrainsKotlinAndroid)
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
        sourceCompatibility = JavaVersion.VERSION_1_8
        targetCompatibility = JavaVersion.VERSION_1_8
    }
    kotlinOptions {
        jvmTarget = "1.8"
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
