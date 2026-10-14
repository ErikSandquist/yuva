package dev.yuva.sample

import android.app.Application
import dev.yuva.analytics.YuvaAnalytics
import dev.yuva.analytics.YuvaAnalyticsConfiguration
import dev.yuva.analytics.YuvaRequestMonitoring
import java.net.URI

class SampleApp : Application() {
    override fun onCreate() {
        super.onCreate()
        if (BuildConfig.SERVER_URL.isNotEmpty() && BuildConfig.ANALYTICS_KEY.isNotEmpty()) {
            YuvaAnalytics.configure(
                this,
                YuvaAnalyticsConfiguration(
                    serverUrl = BuildConfig.SERVER_URL,
                    key = BuildConfig.ANALYTICS_KEY,
                    requireConsent = BuildConfig.ANALYTICS_CONSENT == "required",
                    debug = true,
                    requests = BuildConfig.API_KEY.takeIf { it.isNotEmpty() }?.let {
                        YuvaRequestMonitoring(
                            key = it,
                            hosts = listOfNotNull(URI(BuildConfig.SERVER_URL).host),
                            routes = listOf("/healthz", "/readyz", "/client/v1/conversations/{id}"),
                            environment = "development",
                        )
                    },
                ),
            )
        }
    }
}
