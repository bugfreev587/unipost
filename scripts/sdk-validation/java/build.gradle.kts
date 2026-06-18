plugins {
    application
}

val sdkVersion = providers.gradleProperty("unipostJavaSdkVersion").orElse("0.4.0")
val useLocalSdk = providers.gradleProperty("useLocalSdk").map { it.toBoolean() }.orElse(false)

repositories {
    if (useLocalSdk.get()) {
        mavenLocal()
    }
    mavenCentral()
}

dependencies {
    implementation("dev.unipost:sdk-java:${sdkVersion.get()}")
    testImplementation("org.junit.jupiter:junit-jupiter:5.10.2")
}

application {
    mainClass.set("dev.unipost.validation.UnipostSdkTest")
}

tasks.test {
    useJUnitPlatform()
}

java {
    sourceCompatibility = JavaVersion.VERSION_11
    targetCompatibility = JavaVersion.VERSION_11
}
