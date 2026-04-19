import Foundation
import Metal
import UIKit
import simd

private let maxSampleCount = 512
private let coverageInner: Float = 0.35
private let coverageOuter: Float = 1.10
private let baseAlpha: Float = 195.0 / 255.0

private struct ShaderUniforms {
    var topMercY: Float
    var botMercY: Float
    var leftLon: Float
    var rightLon: Float
    var sampleCount: UInt32
    var coverageInner: Float
    var coverageOuter: Float
    var baseAlpha: Float
}

private struct ShaderSample {
    var coord: SIMD2<Float>  // lat, lon
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

final class TemperatureMetalRenderer {
    private let device: MTLDevice
    private let commandQueue: MTLCommandQueue
    private let pipeline: MTLRenderPipelineState
    private let samplesBuffer: MTLBuffer
    private var uniforms: ShaderUniforms
    private var sampleCount: Int = 0

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
              let fragmentFn = library.makeFunction(name: "temperature_fragment")
        else { return nil }

        let descriptor = MTLRenderPipelineDescriptor()
        descriptor.vertexFunction = vertexFn
        descriptor.fragmentFunction = fragmentFn
        descriptor.colorAttachments[0].pixelFormat = .bgra8Unorm
        descriptor.colorAttachments[0].isBlendingEnabled = true
        descriptor.colorAttachments[0].rgbBlendOperation = .add
        descriptor.colorAttachments[0].alphaBlendOperation = .add
        descriptor.colorAttachments[0].sourceRGBBlendFactor = .one
        descriptor.colorAttachments[0].sourceAlphaBlendFactor = .one
        descriptor.colorAttachments[0].destinationRGBBlendFactor = .oneMinusSourceAlpha
        descriptor.colorAttachments[0].destinationAlphaBlendFactor = .oneMinusSourceAlpha

        guard let pipeline = try? device.makeRenderPipelineState(descriptor: descriptor),
              let samplesBuffer = device.makeBuffer(
                length: MemoryLayout<ShaderSample>.stride * maxSampleCount,
                options: .storageModeShared
              )
        else { return nil }

        self.device = device
        self.commandQueue = queue
        self.pipeline = pipeline
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
    }

    func renderImage(bounds: MercatorBounds, width: Int, height: Int) -> UIImage? {
        guard sampleCount > 0, width > 0, height > 0 else { return nil }

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

        encoder.setRenderPipelineState(pipeline)
        var uniformsCopy = uniforms
        encoder.setFragmentBytes(&uniformsCopy, length: MemoryLayout<ShaderUniforms>.stride, index: 0)
        encoder.setFragmentBuffer(samplesBuffer, offset: 0, index: 1)
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
