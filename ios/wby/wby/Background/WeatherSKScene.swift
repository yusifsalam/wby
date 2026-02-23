import SpriteKit

final class WeatherSKScene: SKScene {

    private var currentWeatherScene: WeatherScene

    init(size: CGSize, weatherScene: WeatherScene) {
        self.currentWeatherScene = weatherScene
        super.init(size: size)
        backgroundColor = .clear
        scaleMode = .resizeFill
    }

    required init?(coder aDecoder: NSCoder) {
        fatalError("init(coder:) not supported")
    }

    override func didMove(to view: SKView) {
        view.allowsTransparency = true
        setupParticles(for: currentWeatherScene)
    }

    func transition(to newScene: WeatherScene) {
        guard newScene != currentWeatherScene else { return }
        currentWeatherScene = newScene
        removeAllChildren()
        removeAllActions()
        setupParticles(for: newScene)
    }

    override func didChangeSize(_ oldSize: CGSize) {
        guard size != oldSize, size != .zero else { return }
        removeAllChildren()
        removeAllActions()
        setupParticles(for: currentWeatherScene)
    }

    // MARK: - Scene Setup

    private func setupParticles(for scene: WeatherScene) {
        switch scene {
        case .clearDay, .overcast:
            break
        case .clearNight:
            addStars(count: 120)
        case .partlyCloudy:
            addClouds(count: 3)
        case .partlyCloudyNight:
            addStars(count: 60)
            addClouds(count: 2)
        case .rain:
            addRain(intensity: 1.0)
        case .snow:
            addSnow()
        case .storm:
            addRain(intensity: 2.0)
            scheduleLightning()
        }
    }

    // MARK: - Stars

    private func addStars(count: Int) {
        for _ in 0..<count {
            let star = SKSpriteNode(color: .white, size: CGSize(width: 2, height: 2))
            star.position = CGPoint(
                x: CGFloat.random(in: 0...size.width),
                y: CGFloat.random(in: size.height * 0.25...size.height)
            )
            star.alpha = CGFloat.random(in: 0.3...0.9)
            addChild(star)

            let minAlpha = CGFloat.random(in: 0.1...0.3)
            let maxAlpha = CGFloat.random(in: 0.6...1.0)
            let duration = TimeInterval.random(in: 1.0...3.0)
            let fadeOut = SKAction.fadeAlpha(to: minAlpha, duration: duration)
            let fadeIn = SKAction.fadeAlpha(to: maxAlpha, duration: duration)
            star.run(.repeatForever(.sequence([fadeOut, fadeIn])))
        }
    }

    // MARK: - Clouds

    private func addClouds(count: Int) {
        for i in 0..<count {
            let cloud = makeCloudNode()
            let xStart = -200 + CGFloat(i) * (size.width / CGFloat(count))
            let yPos = size.height * CGFloat.random(in: 0.60...0.88)
            cloud.position = CGPoint(x: xStart, y: yPos)
            addChild(cloud)

            let travelDuration = TimeInterval.random(in: 40...80)
            let moveAcross = SKAction.moveBy(x: size.width + 400, y: 0, duration: travelDuration)
            let resetPos = SKAction.run { [weak cloud, weak self] in
                guard let cloud, let self else { return }
                cloud.position.x = -200
                cloud.position.y = self.size.height * CGFloat.random(in: 0.60...0.88)
            }
            cloud.run(.repeatForever(.sequence([moveAcross, resetPos])))
        }
    }

    private func makeCloudNode() -> SKNode {
        let container = SKNode()
        let puffs: [(CGFloat, CGFloat, CGFloat)] = [
            (0, 0, 45),
            (-40, -10, 32),
            (40, -10, 36),
            (-20, 12, 26),
            (22, 14, 28),
        ]
        for (x, y, radius) in puffs {
            let puff = SKShapeNode(circleOfRadius: radius)
            puff.fillColor = UIColor.white.withAlphaComponent(0.11)
            puff.strokeColor = .clear
            puff.position = CGPoint(x: x, y: y)
            container.addChild(puff)
        }
        return container
    }

    // MARK: - Rain

    private func addRain(intensity: CGFloat) {
        let emitter = makeRainEmitter(intensity: intensity)
        emitter.position = CGPoint(x: size.width / 2, y: size.height + 20)
        emitter.particlePositionRange = CGVector(dx: size.width * 1.5, dy: 0)
        addChild(emitter)
    }

    private func makeRainEmitter(intensity: CGFloat) -> SKEmitterNode {
        let emitter = SKEmitterNode()
        emitter.particleTexture = WeatherSKScene.rainTexture
        emitter.particleBirthRate = 250 * intensity
        emitter.particleLifetime = 0.8
        emitter.particleLifetimeRange = 0.2
        emitter.particleSpeed = 900
        emitter.particleSpeedRange = 150
        emitter.emissionAngle = -.pi / 2 + 0.15
        emitter.emissionAngleRange = 0.05
        emitter.particleScale = 0.3
        emitter.particleScaleRange = 0.1
        emitter.particleAlpha = 0.35
        emitter.particleAlphaRange = 0.1
        emitter.particleColor = UIColor(white: 0.85, alpha: 1.0)
        emitter.particleColorBlendFactor = 1.0
        emitter.yAcceleration = -200
        emitter.xAcceleration = -80
        return emitter
    }

    private static let rainTexture: SKTexture = {
        let size = CGSize(width: 1, height: 12)
        let renderer = UIGraphicsImageRenderer(size: size)
        let image = renderer.image { ctx in
            UIColor.white.setFill()
            ctx.fill(CGRect(origin: .zero, size: size))
        }
        return SKTexture(image: image)
    }()

    // MARK: - Snow

    private func addSnow() {
        let emitter = makeSnowEmitter()
        emitter.position = CGPoint(x: size.width / 2, y: size.height + 20)
        emitter.particlePositionRange = CGVector(dx: size.width * 1.5, dy: 0)
        addChild(emitter)
    }

    private func makeSnowEmitter() -> SKEmitterNode {
        let emitter = SKEmitterNode()
        emitter.particleTexture = WeatherSKScene.snowTexture
        emitter.particleBirthRate = 60
        emitter.particleLifetime = 5.0
        emitter.particleLifetimeRange = 2.0
        emitter.particleSpeed = 120
        emitter.particleSpeedRange = 40
        emitter.emissionAngle = -.pi / 2
        emitter.emissionAngleRange = 0.6
        emitter.particleScale = 0.15
        emitter.particleScaleRange = 0.08
        emitter.particleAlpha = 0.8
        emitter.particleAlphaRange = 0.2
        emitter.particleColor = .white
        emitter.particleColorBlendFactor = 1.0
        emitter.yAcceleration = -30
        emitter.xAcceleration = -10
        return emitter
    }

    private static let snowTexture: SKTexture = {
        let size = CGSize(width: 10, height: 10)
        let renderer = UIGraphicsImageRenderer(size: size)
        let image = renderer.image { ctx in
            UIColor.white.setFill()
            ctx.cgContext.fillEllipse(in: CGRect(origin: .zero, size: size))
        }
        return SKTexture(image: image)
    }()

    // MARK: - Lightning

    private func scheduleLightning() {
        let wait = SKAction.wait(forDuration: 8, withRange: 10)
        let flash = SKAction.run { [weak self] in self?.flashLightning() }
        run(.repeatForever(.sequence([wait, flash])), withKey: "lightning")
    }

    private func flashLightning() {
        let flash = SKSpriteNode(color: .white, size: size)
        flash.position = CGPoint(x: size.width / 2, y: size.height / 2)
        flash.alpha = 0
        flash.zPosition = 100
        addChild(flash)

        let fadeIn = SKAction.fadeAlpha(to: 0.45, duration: 0.05)
        let hold = SKAction.wait(forDuration: 0.05)
        let fadeOut = SKAction.fadeOut(withDuration: 0.25)
        let remove = SKAction.removeFromParent()
        flash.run(.sequence([fadeIn, hold, fadeOut, remove]))
    }
}
