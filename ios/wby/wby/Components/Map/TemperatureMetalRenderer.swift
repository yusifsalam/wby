import Foundation
import Metal
import simd
import UIKit

private let maxSampleCount = 2048
private let coverageInner: Float = 0.35
private let coverageOuter: Float = 1.10
private let baseAlpha: Float = 217.0 / 255.0

private struct ShaderUniforms {
    var topMercY: Float
    var botMercY: Float
    var leftLon: Float
    var rightLon: Float
    var sampleCount: UInt32
    var coverageInner: Float
    var coverageOuter: Float
    var baseAlpha: Float
    var gridMinLat: Float = 0
    var gridMaxLat: Float = 0
    var gridMinLon: Float = 0
    var gridMaxLon: Float = 0
    var gridRows: Float = 0
    var gridCols: Float = 0
}

private struct ShaderSample {
    var coord: SIMD2<Float> // lat, lon
    var temp: Float
    var padding: Float
}

struct MercatorBounds: Equatable {
    var topMercY: Double
    var botMercY: Double
    var leftLon: Double
    var rightLon: Double

    static let finland = MercatorBounds(
        topMercY: mercatorY(lat: 71.5),
        botMercY: mercatorY(lat: 59.0),
        leftLon: 19.0,
        rightLon: 32.0
    )

    static func mercatorY(lat: Double) -> Double {
        let clamped = min(max(lat, -85.05112878), 85.05112878)
        let rad = clamped * .pi / 180.0
        return log(tan(.pi / 4 + rad / 2))
    }
}

private enum RenderMode {
    case points
    case grid
}

/// Which field a grid texture holds, selecting the colour ramp / alpha shader.
enum GridField {
    case temperature
    case precipitation
}

final class TemperatureMetalRenderer {
    private let device: MTLDevice
    private let commandQueue: MTLCommandQueue
    private let pipeline: MTLRenderPipelineState
    private let gridPipeline: MTLRenderPipelineState
    private let gridPrecipPipeline: MTLRenderPipelineState
    private let samplesBuffer: MTLBuffer
    private var uniforms: ShaderUniforms
    private var sampleCount: Int = 0
    private var mode: RenderMode = .points
    private var gridField: GridField = .temperature
    private var fieldTexture: MTLTexture?

    init?() {
        guard let device = MTLCreateSystemDefaultDevice(),
              let queue = device.makeCommandQueue()
        else { return nil }

        let library: MTLLibrary
        do {
            library = try device.makeDefaultLibrary(bundle: .main)
        } catch {
            return nil
        }

        guard let vertexFn = library.makeFunction(name: "temperature_vertex"),
              let fragmentFn = library.makeFunction(name: "temperature_fragment"),
              let gridFragmentFn = library.makeFunction(name: "temperature_grid_fragment"),
              let gridPrecipFragmentFn = library.makeFunction(name: "precipitation_grid_fragment")
        else { return nil }

        func makePipeline(fragment: MTLFunction) -> MTLRenderPipelineState? {
            let descriptor = MTLRenderPipelineDescriptor()
            descriptor.vertexFunction = vertexFn
            descriptor.fragmentFunction = fragment
            descriptor.colorAttachments[0].pixelFormat = .bgra8Unorm
            descriptor.colorAttachments[0].isBlendingEnabled = true
            descriptor.colorAttachments[0].rgbBlendOperation = .add
            descriptor.colorAttachments[0].alphaBlendOperation = .add
            descriptor.colorAttachments[0].sourceRGBBlendFactor = .one
            descriptor.colorAttachments[0].sourceAlphaBlendFactor = .one
            descriptor.colorAttachments[0].destinationRGBBlendFactor = .oneMinusSourceAlpha
            descriptor.colorAttachments[0].destinationAlphaBlendFactor = .oneMinusSourceAlpha
            return try? device.makeRenderPipelineState(descriptor: descriptor)
        }

        guard let pipeline = makePipeline(fragment: fragmentFn),
              let gridPipeline = makePipeline(fragment: gridFragmentFn),
              let gridPrecipPipeline = makePipeline(fragment: gridPrecipFragmentFn),
              let samplesBuffer = device.makeBuffer(
                  length: MemoryLayout<ShaderSample>.stride * maxSampleCount,
                  options: .storageModeShared
              )
        else { return nil }

        self.device = device
        self.commandQueue = queue
        self.pipeline = pipeline
        self.gridPipeline = gridPipeline
        self.gridPrecipPipeline = gridPrecipPipeline
        self.samplesBuffer = samplesBuffer
        self.uniforms = ShaderUniforms(
            topMercY: Float(MercatorBounds.finland.topMercY),
            botMercY: Float(MercatorBounds.finland.botMercY),
            leftLon: Float(MercatorBounds.finland.leftLon),
            rightLon: Float(MercatorBounds.finland.rightLon),
            sampleCount: 0,
            coverageInner: coverageInner,
            coverageOuter: coverageOuter,
            baseAlpha: baseAlpha
        )
    }

    func setSamples(_ samples: [TemperatureSample]) {
        let capped = Array(samples.prefix(maxSampleCount))
        let pointer = samplesBuffer.contents().bindMemory(to: ShaderSample.self, capacity: maxSampleCount)
        for (i, sample) in capped.enumerated() {
            pointer[i] = ShaderSample(
                coord: SIMD2<Float>(Float(sample.lat), Float(sample.lon)),
                temp: Float(sample.temp),
                padding: 0
            )
        }
        sampleCount = capped.count
        uniforms.sampleCount = UInt32(capped.count)
        mode = .points
    }

    /// Uploads a regular lat/lon field raster as an rg16Float texture
    /// (r = value·valid, g = valid) for hardware-bilinear sampling. `field`
    /// selects the colour ramp / alpha shader. Returns false if the grid is
    /// malformed or the texture can't be allocated.
    @discardableResult
    func setGrid(_ grid: TemperatureGrid, field: GridField = .temperature) -> Bool {
        guard grid.rows > 0, grid.cols > 0,
              grid.values.count == grid.rows * grid.cols else { return false }

        let descriptor = MTLTextureDescriptor.texture2DDescriptor(
            pixelFormat: .rg16Float,
            width: grid.cols,
            height: grid.rows,
            mipmapped: false
        )
        descriptor.usage = [.shaderRead]
        descriptor.storageMode = .shared
        guard let texture = device.makeTexture(descriptor: descriptor) else { return false }

        var texels = [Float16](repeating: 0, count: grid.rows * grid.cols * 2)
        for (i, value) in grid.values.enumerated() {
            if let value {
                texels[i * 2] = Float16(value)      // temp · valid
                texels[i * 2 + 1] = 1                // valid
            }
            // nil cell stays (0, 0): zero weight, excluded by normalized convolution.
        }
        texels.withUnsafeBytes { raw in
            texture.replace(
                region: MTLRegionMake2D(0, 0, grid.cols, grid.rows),
                mipmapLevel: 0,
                withBytes: raw.baseAddress!,
                bytesPerRow: grid.cols * 2 * MemoryLayout<Float16>.stride
            )
        }

        fieldTexture = texture
        uniforms.gridMinLat = Float(grid.minLat)
        uniforms.gridMaxLat = Float(grid.maxLat)
        uniforms.gridMinLon = Float(grid.minLon)
        uniforms.gridMaxLon = Float(grid.maxLon)
        uniforms.gridRows = Float(grid.rows)
        uniforms.gridCols = Float(grid.cols)
        gridField = field
        mode = .grid
        return true
    }

    func renderImage(bounds: MercatorBounds, width: Int, height: Int) -> UIImage? {
        guard width > 0, height > 0 else { return nil }
        switch mode {
        case .points where sampleCount > 0: break
        case .grid where fieldTexture != nil: break
        default: return nil
        }

        uniforms.topMercY = Float(bounds.topMercY)
        uniforms.botMercY = Float(bounds.botMercY)
        uniforms.leftLon = Float(bounds.leftLon)
        uniforms.rightLon = Float(bounds.rightLon)

        let textureDescriptor = MTLTextureDescriptor.texture2DDescriptor(
            pixelFormat: .bgra8Unorm,
            width: width,
            height: height,
            mipmapped: false
        )
        textureDescriptor.usage = [.renderTarget]
        textureDescriptor.storageMode = .shared
        guard let texture = device.makeTexture(descriptor: textureDescriptor),
              let commandBuffer = commandQueue.makeCommandBuffer()
        else { return nil }
        let passDescriptor = MTLRenderPassDescriptor()

        passDescriptor.colorAttachments[0].texture = texture
        passDescriptor.colorAttachments[0].loadAction = .clear
        passDescriptor.colorAttachments[0].storeAction = .store
        passDescriptor.colorAttachments[0].clearColor = MTLClearColor(red: 0, green: 0, blue: 0, alpha: 0)

        guard let encoder = commandBuffer.makeRenderCommandEncoder(descriptor: passDescriptor) else {
            return nil
        }

        var uniformsCopy = uniforms
        switch mode {
        case .points:
            encoder.setRenderPipelineState(pipeline)
            encoder.setFragmentBytes(&uniformsCopy, length: MemoryLayout<ShaderUniforms>.stride, index: 0)
            encoder.setFragmentBuffer(samplesBuffer, offset: 0, index: 1)
        case .grid:
            encoder.setRenderPipelineState(gridField == .precipitation ? gridPrecipPipeline : gridPipeline)
            encoder.setFragmentBytes(&uniformsCopy, length: MemoryLayout<ShaderUniforms>.stride, index: 0)
            encoder.setFragmentTexture(fieldTexture, index: 0)
        }
        encoder.drawPrimitives(type: .triangle, vertexStart: 0, vertexCount: 3)
        encoder.endEncoding()

        commandBuffer.commit()
        commandBuffer.waitUntilCompleted()
        guard commandBuffer.status == .completed else { return nil }

        let bytesPerRow = width * 4
        var bytes = [UInt8](repeating: 0, count: bytesPerRow * height)
        texture.getBytes(
            &bytes,
            bytesPerRow: bytesPerRow,
            from: MTLRegionMake2D(0, 0, width, height),
            mipmapLevel: 0
        )

        let colorSpace = CGColorSpaceCreateDeviceRGB()
        let bitmapInfo = CGBitmapInfo.byteOrder32Little.union(
            CGBitmapInfo(rawValue: CGImageAlphaInfo.premultipliedFirst.rawValue)
        )
        guard let provider = CGDataProvider(data: Data(bytes) as CFData),
              let image = CGImage(
                  width: width,
                  height: height,
                  bitsPerComponent: 8,
                  bitsPerPixel: 32,
                  bytesPerRow: bytesPerRow,
                  space: colorSpace,
                  bitmapInfo: bitmapInfo,
                  provider: provider,
                  decode: nil,
                  shouldInterpolate: false,
                  intent: .defaultIntent
              )
        else { return nil }

        return UIImage(cgImage: image)
    }
}
