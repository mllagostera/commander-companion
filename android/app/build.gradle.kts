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

        // Base URL del backend. 10.0.2.2 es el alias que el emulador de Android usa para
        // llegar a "localhost" de la máquina host (donde corre `go run ./cmd/api` en dev).
        // Override sin tocar código: `./gradlew :app:assembleDebug -PAPI_BASE_URL=http://192.168.1.50:8080/`
        // (para dispositivo físico en la misma red) o seteando la propiedad en gradle.properties.
        val apiBaseUrl = (project.findProperty("API_BASE_URL") as String?)
            ?: "http://10.0.2.2:8080/"
        buildConfigField("String", "API_BASE_URL", "\"$apiBaseUrl\"")

        // PLACEHOLDER: el Web Client ID real todavía no existe (ver docs/roadmap/TASKS.md Stage 1,
        // "Crear credenciales OAuth en Google Cloud Console" — paso manual externo, pendiente).
        // Hasta que se cree, Credential Manager va a fallar el flujo de Google Sign-In con este
        // placeholder porque Google no reconoce el client ID. Override: -PGOOGLE_WEB_CLIENT_ID=...
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
            // Los tests unitarios tocan tipos con dependencias sueltas del framework
            // (p. ej. android.util.Log vía Room/Compose); sin esto lanzan RuntimeException
            // "not mocked" en vez de devolver el default del tipo.
            isReturnDefaultValues = true
        }
    }
}

dependencies {
    implementation(libs.androidx.core.ktx)
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

    // OkHttp (cliente HTTP subyacente de Retrofit): interceptor de auth + logging
    implementation(libs.okhttp)
    implementation(libs.okhttp.logging.interceptor)

    // DataStore: persistencia de sesión (access/refresh token + expiry)
    implementation(libs.androidx.datastore.preferences)

    // Credential Manager + Google Identity Services: Google Sign-In real
    implementation(libs.androidx.credentials)
    implementation(libs.androidx.credentials.play.services.auth)
    implementation(libs.googleid)

    testImplementation(libs.junit)
    // runTest + Dispatchers.setMain para testear ViewModels y repositorios con corrutinas.
    testImplementation(libs.kotlinx.coroutines.test)
    androidTestImplementation(libs.androidx.junit)
    androidTestImplementation(libs.androidx.espresso.core)
    androidTestImplementation(platform(libs.androidx.compose.bom))
    androidTestImplementation(libs.androidx.ui.test.junit4)
    debugImplementation(libs.androidx.ui.tooling)
    debugImplementation(libs.androidx.ui.test.manifest)
}
