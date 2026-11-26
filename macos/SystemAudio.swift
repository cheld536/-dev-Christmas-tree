import AVFoundation
import Foundation
import ScreenCaptureKit

@available(macOS 13.0, *)
final class SystemAudio: NSObject, SCStreamOutput, SCStreamDelegate {
    private let captureQueue = DispatchQueue(label: "DeveloperChristmasTree.audio")
    private let lock = NSLock()
    private var stream: SCStream?
    private var pending = Data()
    private var sampleRate = 0
    private let stopped: (SystemAudio) -> Void

    init(stopped: @escaping (SystemAudio) -> Void) {
        self.stopped = stopped
    }

    func start() async throws {
        let content = try await SCShareableContent.excludingDesktopWindows(false, onScreenWindowsOnly: true)
        guard let display = content.displays.first else { throw CaptureError.noDisplay }
        let configuration = SCStreamConfiguration()
        configuration.capturesAudio = true
        configuration.excludesCurrentProcessAudio = true
        configuration.sampleRate = 48_000
        configuration.channelCount = 1
        let active = SCStream(
            filter: SCContentFilter(display: display, excludingWindows: []),
            configuration: configuration, delegate: self)
        try active.addStreamOutput(self, type: .audio, sampleHandlerQueue: captureQueue)
        try await active.startCapture()
        stream = active
    }

    func stop() async {
        if let active = stream {
            stream = nil
            try? await active.stopCapture()
        }
    }

    func drain() -> (Data, Int) {
        lock.lock()
        defer { lock.unlock() }
        let audio = pending
        pending.removeAll(keepingCapacity: false)
        return (audio, sampleRate)
    }

    func stream(_ stream: SCStream, didStopWithError error: Error) {
        DispatchQueue.main.async { [weak self] in
            guard let self else { return }
            self.stopped(self)
        }
    }

    func stream(_ stream: SCStream, didOutputSampleBuffer sampleBuffer: CMSampleBuffer, of type: SCStreamOutputType) {
        guard type == .audio, sampleBuffer.isValid,
              let format = sampleBuffer.formatDescription?.audioStreamBasicDescription,
              format.mChannelsPerFrame == 1, format.mBitsPerChannel == 32,
              format.mFormatFlags & kAudioFormatFlagIsFloat != 0 else { return }
        try? sampleBuffer.withAudioBufferList { buffers, _ in
            guard let buffer = buffers.first, let bytes = buffer.mData else { return }
            let count = Int(buffer.mDataByteSize)
            let capacity = 4096 * MemoryLayout<Float>.size
            guard count > 0 && count <= capacity else { return }
            lock.lock()
            defer { lock.unlock() }
            if pending.count + count > capacity { pending.removeAll(keepingCapacity: true) }
            pending.append(bytes.assumingMemoryBound(to: UInt8.self), count: count)
            sampleRate = Int(format.mSampleRate)
        }
    }

    private enum CaptureError: Error {
        case noDisplay
    }
}
