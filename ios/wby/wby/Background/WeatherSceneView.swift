import SwiftUI
import SpriteKit

struct WeatherSceneView: View {
    let weatherScene: WeatherScene

    @State private var skScene: WeatherSKScene?

    var body: some View {
        GeometryReader { geo in
            Group {
                if let skScene {
                    SpriteView(scene: skScene, options: [.allowsTransparency])
                }
            }
            .onAppear {
                guard skScene == nil else { return }
                skScene = WeatherSKScene(size: geo.size, weatherScene: weatherScene)
            }
            .onChange(of: weatherScene) { _, newScene in
                skScene?.transition(to: newScene)
            }
        }
    }
}
