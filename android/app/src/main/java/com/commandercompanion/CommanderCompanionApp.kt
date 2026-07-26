package com.commandercompanion

import android.app.Application
import dagger.hilt.android.HiltAndroidApp

@HiltAndroidApp
class CommanderCompanionApp : Application() {
    override fun onCreate() {
        super.onCreate()
    }
}
