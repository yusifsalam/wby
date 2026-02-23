import SwiftUI
import SpriteKit

struct WeatherSceneView: View {
    let weatherScene: WeatherScene
    let precipitation1h: Double?

    @State private var skScene: WeatherSKScene?

    var body: some View {
        ZStack {
            if let skScene {
                SpriteView(scene: skScene, options: [.allowsTransparency])
            }
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .onAppear {
            guard skScene == nil else { return }
            skScene = WeatherSKScene(
                size: UIScreen.main.bounds.size,
                weatherScene: weatherScene,
                precipitation1h: precipitation1h
            )
        }
        .onChange(of: weatherScene) { _, newScene in
            skScene?.transition(to: newScene, precipitation1h: precipitation1h)
        }
        .onChange(of: precipitation1h) { _, newPrecip in
            skScene?.transition(to: weatherScene, precipitation1h: newPrecip)
        }
    }
}
