plugins {
    application
}

val sdkVersion = providers.gradleProperty("unipostJavaSdkVersion").orElse("0.2.5")
val useLocalSdk = providers.gradleProperty("useLocalSdk").map { it.toBoolean() }.orElse(false)

repositories {
    if (useLocalSdk.get()) {
        mavenLocal()
    }
    mavenCentral()
}

dependencies {
    implementation("dev.unipost:sdk-java:${sdkVersion.get()}")
}

application {
    mainClass.set("dev.unipost.validation.UnipostSdkTest")
}

java {
    sourceCompatibility = JavaVersion.VERSION_11
    targetCompatibility = JavaVersion.VERSION_11
}
