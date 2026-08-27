#!/bin/bash
set -e

APP_NAME="tcno-acc-switcher.exe"
BUNDLE_ID="com.example.tcnoaccswitcher"
VERSION="0.1.0"
BUILD_NUMBER="0.1.0"
BUILD_DIR="build/ios"
TARGET="simulator"

echo "Building iOS app: $APP_NAME"
echo "Bundle ID: $BUNDLE_ID"
echo "Version: $VERSION ($BUILD_NUMBER)"
echo "Target: $TARGET"

mkdir -p "$BUILD_DIR"

if [ "$TARGET" = "simulator" ]; then
    SDK="iphonesimulator"
    ARCH="arm64-apple-ios15.0-simulator"
elif [ "$TARGET" = "device" ]; then
    SDK="iphoneos"
    ARCH="arm64-apple-ios15.0"
else
    echo "Unknown target: $TARGET"
    exit 1
fi

SDK_PATH=$(xcrun --sdk $SDK --show-sdk-path)

echo "Compiling with SDK: $SDK"
xcrun -sdk $SDK clang \
    -target $ARCH \
    -isysroot "$SDK_PATH" \
    -framework Foundation \
    -framework UIKit \
    -framework WebKit \
    -framework CoreGraphics \
    -o "$BUILD_DIR/$APP_NAME" \
    "$BUILD_DIR/main.m"

echo "Creating app bundle..."
APP_BUNDLE="$BUILD_DIR/$APP_NAME.app"
rm -rf "$APP_BUNDLE"
mkdir -p "$APP_BUNDLE"

mv "$BUILD_DIR/$APP_NAME" "$APP_BUNDLE/"

cp "$BUILD_DIR/Info.plist" "$APP_BUNDLE/"

echo "Signing app..."
codesign --force --sign - "$APP_BUNDLE"

echo "Build complete: $APP_BUNDLE"

if [ "$TARGET" = "simulator" ]; then
    echo "Deploying to simulator..."
    xcrun simctl terminate booted "$BUNDLE_ID" 2>/dev/null || true
    xcrun simctl install booted "$APP_BUNDLE"
    xcrun simctl launch booted "$BUNDLE_ID"
    echo "App launched on simulator"
fi