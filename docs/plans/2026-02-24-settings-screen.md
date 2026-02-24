# Settings Screen Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a settings screen where the user can toggle SpriteKit particle effects on/off, persisted via UserDefaults.

**Architecture:** Wrap `ContentView.body` in a `NavigationStack`; add a gear toolbar button that pushes `SettingsView`. Both views use `@AppStorage("dynamicEffectsEnabled")` on the same key — no binding passing needed. The gradient always renders; `WeatherSceneView` is conditionally rendered based on the flag.

**Tech Stack:** SwiftUI, `@AppStorage` (UserDefaults), `NavigationStack`, SpriteKit (existing)

---

### Task 1: Create SettingsView

**Files:**
- Create: `ios/wby/wby/Views/SettingsView.swift`

No unit tests for this pure UI view.

**Step 1: Create the file**

```swift
import SwiftUI

struct SettingsView: View {
    @AppStorage("dynamicEffectsEnabled") private var dynamicEffectsEnabled = true

    var body: some View {
        Form {
            Section {
                Toggle("Dynamic Background Effects", isOn: $dynamicEffectsEnabled)
            } footer: {
                Text("Shows rain, snow, and other animated weather effects in the background.")
            }
        }
        .navigationTitle("Settings")
        .navigationBarTitleDisplayMode(.large)
    }
}

#Preview {
    NavigationStack {
        SettingsView()
    }
}
```

**Step 2: Verify it builds**

```bash
xcodebuild -project ios/wby/wby.xcodeproj -scheme wby \
  -destination 'generic/platform=iOS Simulator' build \
  2>&1 | tail -5
```

Expected: `** BUILD SUCCEEDED **`

**Step 3: Commit**

```bash
git add ios/wby/wby/Views/SettingsView.swift
git commit -m "ios: add SettingsView with dynamic effects toggle"
```

---

### Task 2: Update ContentView

**Files:**
- Modify: `ios/wby/wby/ContentView.swift`

**Step 1: Add `@AppStorage` and `showSettings` state at the top of `ContentView`**

In `ContentView`, after the existing `@State` declarations, add:

```swift
@AppStorage("dynamicEffectsEnabled") private var dynamicEffectsEnabled = true
```

**Step 2: Wrap `body` in `NavigationStack` and add toolbar**

Replace the `var body: some View` implementation:

```swift
var body: some View {
    NavigationStack {
        ZStack {
            mainBackground
            ScrollView {
                VStack(spacing: 8) {
                    // ... (unchanged content) ...
                }
                .padding()
            }
            .scrollBounceBehavior(.always)
            .refreshable { await loadWeather() }
            .toolbar {
                ToolbarItem(placement: .topBarTrailing) {
                    NavigationLink {
                        SettingsView()
                    } label: {
                        Image(systemName: "gearshape")
                            .foregroundStyle(
                                currentScene.prefersLightForeground ? .white : .primary
                            )
                    }
                }
            }
            .toolbarBackground(.clear, for: .navigationBar)
            .navigationTitle("")
        }
        .task {
            guard !disableAutoLoad else { return }
            await loadWeather()
        }
        .onChange(of: locationService.coordinate?.latitude) {
            guard !disableAutoLoad else { return }
            Task { await loadWeather() }
        }
    }
}
```

Note: the `.task` and `.onChange` modifiers move from `ZStack` to inside `NavigationStack` on the `ZStack` — same effect, just relocated to work with the new structure.

**Step 3: Make `WeatherSceneView` conditional in `mainBackground`**

In the `mainBackground` computed property, wrap `WeatherSceneView` with a guard:

```swift
private var mainBackground: some View {
    ZStack {
        LinearGradient(
            colors: currentScene.gradientColors,
            startPoint: .top,
            endPoint: .bottom
        )
        .ignoresSafeArea()
        .id(currentScene)
        .transition(.opacity)

        if dynamicEffectsEnabled {
            WeatherSceneView(
                weatherScene: currentScene,
                precipitation1h: weather?.hourlyForecast.first?.precipitation1h
            )
            .ignoresSafeArea()
        }
    }
    .animation(.easeInOut(duration: 1.5), value: currentScene)
}
```

**Step 4: Verify it builds**

```bash
xcodebuild -project ios/wby/wby.xcodeproj -scheme wby \
  -destination 'generic/platform=iOS Simulator' build \
  2>&1 | tail -5
```

Expected: `** BUILD SUCCEEDED **`

**Step 5: Commit**

```bash
git add ios/wby/wby/ContentView.swift
git commit -m "ios: add settings navigation and dynamic effects toggle"
```
