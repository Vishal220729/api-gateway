# 📱 API Gateway Mobile App Guide

The API Gateway is engineered as a **Responsive Web Design (RWD) Progressive Web App (PWA)** and **Native-Ready Mobile Application**.

---

## ⚡ Option 1: Instant PWA Mobile Install (Recommended - No Store Required)

You can install the app on your phone **right now** directly from your live AWS URL:

### 🤖 On Android (Chrome / Brave / Edge):
1. Open Chrome on your phone and navigate to:
   ```
   http://56.228.11.97:8080/intro.html
   ```
2. You will see the floating **📲 Install** banner at the top, or tap the three dots `⋮` at the top right.
3. Tap **"Install app"** or **"Add to Home screen"**.
4. The app will install with the custom dark cloud icon, launch in fullscreen standalone mode (no browser address bar), and work with the native mobile bottom navigation bar!

### 🍏 On iOS (Safari):
1. Open Safari on your iPhone and navigate to:
   ```
   http://56.228.11.97:8080/intro.html
   ```
2. Tap the **Share** button (box with upward arrow) at the bottom.
3. Scroll down and tap **"Add to Home Screen"**.
4. Tap **Add**. The app will be placed on your home screen and run in standalone fullscreen mode with notch safe-area insets!

---

## 📦 Option 2: Build a Standalone Android APK (.apk) with Capacitor

If you want a native `.apk` file to share or upload to the Google Play Store:

### Prerequisites:
- Node.js installed (`node -v`)
- Android Studio installed with Android SDK

### Steps to Build APK:
```bash
# 1. Navigate to the mobile directory
cd mobile

# 2. Install Capacitor dependencies
npm init -y
npm install @capacitor/core @capacitor/cli @capacitor/android

# 3. Initialize and add Android platform
npx cap add android

# 4. Sync web assets & configuration
npx cap sync android

# 5. Open in Android Studio to build APK
npx cap open android
# In Android Studio: Build -> Build Bundle(s) / APK(s) -> Build APK(s)
```

The compiled APK will be generated at:
`android/app/build/outputs/apk/debug/app-debug.apk`.
