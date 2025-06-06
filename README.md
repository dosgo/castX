# CastX - Android Screen Mirroring & Control

[View in Chinese](README_zh.md)|[中文文档](README_zh.md)

CastX is an open-source Android screen mirroring solution with two operating modes:
1. **Device Mode**: Run on an Android device to enable browser-based control via HTTP/WebRTC in your local network
2. **Desktop Mode**: Install on computers to control other Android devices via scrcpy

---

## ✨ Features
- **Cross-platform support** (Android, Windows, macOS, Linux)
- Browser-based device control (no client installation required)
- Low-latency screen mirroring via WebRTC
- Secure local network operation
- Intuitive web interface with virtual control buttons
- scrcpy integration for advanced control



## 🚀 Device Mode - Android Installation
1. Install the APK on your Android device
2. Launch CastX and grant necessary permissions
3. Tap "Start Server"
4. Access the control panel from any browser at:http://:8080
5. Use the web interface to:
   - View device screen in real-time
   - Send touch events and gestures
   - Control volume and power states


## 💻 Desktop Mode - Computer Installation
1. Install the CastX desktop application
2. Connect target Android device via USB or Wi-Fi
3. Enable USB debugging on the Android device
4. Launch CastX Desktop
5. Select device from detected list
6. Control device with keyboard/mouse