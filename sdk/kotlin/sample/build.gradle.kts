import com.android.build.api.artifact.SingleArtifact
import java.util.Properties

plugins {
    alias(libs.plugins.android.application)
    alias(libs.plugins.kotlin.compose)
}

val sampleConfig = Properties().apply {
    val file = rootProject.file("sample/yuva.properties")
    if (file.isFile) file.inputStream().use(::load)
}

fun configValue(key: String) = "\"" + sampleConfig.getProperty(key, "").replace("\"", "\\\"") + "\""

android {
    namespace = "dev.yuva.sample"
    compileSdk = 37
    defaultConfig {
        applicationId = "dev.yuva.sample"
        minSdk = 26
        targetSdk = 37
        versionCode = 1
        versionName = providers.gradleProperty("yuva.version").get()
        buildConfigField("String", "SERVER_URL", configValue("serverUrl"))
        buildConfigField("String", "CHANNEL_KEY", configValue("channelKey"))
        buildConfigField("String", "IDENTITY_TOKEN_URL", configValue("identityTokenUrl"))
        buildConfigField("String", "ANALYTICS_KEY", configValue("analyticsKey"))
        buildConfigField("String", "ANALYTICS_CONSENT", configValue("analyticsConsent"))
        buildConfigField("String", "API_KEY", configValue("apiKey"))
    }
    buildFeatures {
        compose = true
        buildConfig = true
    }
    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
    androidResources {
        localeFilters += listOf("en", "tr")
    }
}

dependencies {
    implementation(project(":yuva"))
    implementation(project(":analytics"))
    implementation(platform(libs.androidx.compose.bom))
    implementation(libs.androidx.compose.ui)
    implementation(libs.androidx.compose.material3)
    implementation(libs.androidx.compose.material.icons.core)
    implementation(libs.androidx.activity.compose)
    implementation(libs.androidx.core.ktx)
}

abstract class CheckNoAdId : DefaultTask() {
    @get:InputFile
    @get:PathSensitive(PathSensitivity.NONE)
    abstract val manifest: RegularFileProperty

    @TaskAction
    fun check() {
        val file = manifest.get().asFile
        check("com.google.android.gms.permission.AD_ID" !in file.readText()) { "$file asks for the advertising id (AD_ID)" }
    }
}

androidComponents {
    onVariants { variant ->
        val task = tasks.register<CheckNoAdId>("check${variant.name.replaceFirstChar(Char::uppercase)}NoAdId") {
            manifest.set(variant.artifacts.get(SingleArtifact.MERGED_MANIFEST))
        }
        tasks.named("check") { dependsOn(task) }
    }
}
