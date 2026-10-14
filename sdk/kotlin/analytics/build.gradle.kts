plugins {
    alias(libs.plugins.android.library)
    alias(libs.plugins.kotlin.compose)
    alias(libs.plugins.kotlin.serialization)
    `maven-publish`
}

android {
    namespace = "dev.yuva.analytics"
    compileSdk = 37
    defaultConfig {
        minSdk = 26
    }
    buildFeatures {
        compose = true
    }
    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
    lint {
        abortOnError = true
        disable += setOf("GradleDependency", "NewerVersionAvailable", "AndroidGradlePluginVersion")
    }
    publishing {
        singleVariant("release") {
            withSourcesJar()
        }
    }
}

dependencies {
    implementation(platform(libs.androidx.compose.bom))
    implementation(libs.androidx.compose.runtime)
    implementation(libs.androidx.lifecycle.process)
    implementation(libs.kotlinx.serialization.json)
    implementation(libs.kotlinx.coroutines.android)
    compileOnly(libs.okhttp)
    testImplementation(libs.junit)
    testImplementation(libs.okhttp)
}

afterEvaluate {
    publishing {
        publications {
            register<MavenPublication>("release") {
                from(components["release"])
                groupId = "com.github.productdevbook"
                artifactId = "yuva-analytics"
                version = providers.environmentVariable("VERSION")
                    .orElse(providers.gradleProperty("yuva.version")).get()
                pom {
                    name.set("Yuva Analytics for Android")
                    description.set("Cookieless app analytics for Yuva: screens, events and revenue, with no advertising id.")
                    url.set("https://github.com/productdevbook/yuva")
                    licenses {
                        license {
                            name.set("MIT")
                            url.set("https://github.com/productdevbook/yuva/blob/main/sdk/LICENSE")
                        }
                    }
                    scm {
                        url.set("https://github.com/productdevbook/yuva")
                        connection.set("scm:git:https://github.com/productdevbook/yuva.git")
                    }
                }
            }
        }
    }
}
