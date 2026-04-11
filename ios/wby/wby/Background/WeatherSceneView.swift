import SwiftUI
import SpriteKit

struct WeatherSceneView: View {
    let weatherScene: WeatherScene
    let precipitation1h: Double?

    @State private var skScene: WeatherSKScene?

    var body: some View {
        GeometryReader { proxy in
            ZStack {
                if let skScene {
                    SpriteView(scene: skScene, options: [.allowsTransparency])
                }
            }
            .frame(maxWidth: .infinity, maxHeight: .infinity)
            .onAppear {
                ensureScene(for: proxy.size)
            }
            .onChange(of: proxy.size) { _, newSize in
                ensureScene(for: newSize)
            }
            .onChange(of: weatherScene) { _, newScene in
                skScene?.transition(to: newScene, precipitation1h: precipitation1h)
            }
            .onChange(of: precipitation1h) { _, newPrecip in
                skScene?.transition(to: weatherScene, precipitation1h: newPrecip)
            }
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }

    private func ensureScene(for size: CGSize) {
        guard size.width > 0, size.height > 0 else { return }
        if let skScene {
            if skScene.size != size {
                skScene.size = size
            }
            return
        }
        self.skScene = WeatherSKScene(
            size: size,
            weatherScene: weatherScene,
            precipitation1h: precipitation1h
        )
    }
}
