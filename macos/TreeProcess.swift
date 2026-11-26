import AppKit
import Combine
import Darwin
import Foundation

struct CoupleSettings: Codable {
    var Enabled = false
    var FirstName = ""
    var SecondName = ""
    var StartDate = ""
}

struct SceneSettings: Codable {
    var Theme = "Rainbow RGB"
    var Direction = "Clockwise"
    var Speed = 100
    var LightSpeed = 100
    var Size = 88
    var Snow = true
    var Ornaments = true
    var Matrix = true
    var Bloom = true
    var Messages = true
    var Countdown = true
    var Stories = true
    var Message = ""
    var Couple = CoupleSettings()
}

struct TreeFrame: Decodable {
    struct Cell: Decodable {
        let X: Int
        let Y: Int
        let Glyph: String
        let Color: UInt32
        let Glow: Bool
    }

    let Columns: Int
    let Rows: Int
    let Background: UInt32
    let Cells: [Cell]
    let Greeting: String
    let FirstName: String
    let SecondName: String
    let Counter: String
    let CoupleError: String
    let AudioActive: Bool
    let AudioBars: [Double]?
}

private struct FrameRequest: Encodable {
    let Columns: Int
    let Rows: Int
    let Elapsed: Double
    let Angle: Double
    let StoryPreview: Int
    let StoryElapsed: Double
    let AudioPCM: Data
    let AudioRate: Int
    let AudioEnabled: Bool
    let Equalizer: String
    let Settings: SceneSettings
}

final class TreeProcess: ObservableObject {
    @Published var settings: SceneSettings {
        didSet {
            if !smokeTest, let saved = try? JSONEncoder().encode(settings) {
                UserDefaults.standard.set(saved, forKey: "TreeSettings")
            }
        }
    }
    @Published private(set) var frame: TreeFrame?
    @Published private(set) var error: String?
    @Published private(set) var storyTitle = ""
    @Published private(set) var musicEnabled = false
    @Published private(set) var audioChanging = false
    @Published private(set) var audioError = ""
    @Published var equalizer = UserDefaults.standard.string(forKey: "TreeEqualizer") ?? "ring" {
        didSet { UserDefaults.standard.set(equalizer, forKey: "TreeEqualizer") }
    }

    private let process = Process()
    private let input = Pipe()
    private let output = Pipe()
    private var buffer = Data()
    private var timer: Timer?
    private var pending = false
    private var sentAt = 0.0
    private var canvasSize = CGSize(width: 680, height: 560)
    private let start = ProcessInfo.processInfo.systemUptime
    private var lastTick = ProcessInfo.processInfo.systemUptime
    private var angle = 0.0
    private var previewStart = 0.0
    private var storyIndex = 0
    private var capture: AnyObject?
    private var smokeStage = 0
    private let smokeTest = CommandLine.arguments.contains("--smoke-test")

    private static let stories = [
        ("산타와 루돌프", 8.0), ("트리 아래 선물", 12.0),
        ("성냥불과 작은 손님", 12.0), ("눈사람의 목도리", 12.0),
        ("겨울 하늘의 별똥별", 8.0),
    ]

    init() {
        if !smokeTest, let data = UserDefaults.standard.data(forKey: "TreeSettings"),
           let saved = try? JSONDecoder().decode(SceneSettings.self, from: data) {
            settings = saved
        } else {
            settings = SceneSettings()
        }
    }

    func begin() {
        guard timer == nil, error == nil else { return }
        process.executableURL = Bundle.main.bundleURL
            .appendingPathComponent("Contents/Helpers/tree-renderer")
        process.standardInput = input
        process.standardOutput = output
        do {
            try process.run()
        } catch {
            fail("트리 엔진을 실행할 수 없습니다. 앱을 다시 압축 해제해 주세요.")
            return
        }
        output.fileHandleForReading.readabilityHandler = { [weak self] handle in
            let bytes = handle.availableData
            if bytes.isEmpty {
                handle.readabilityHandler = nil
            }
            DispatchQueue.main.async { self?.receive(bytes) }
        }
        let timer = Timer(timeInterval: 1.0 / 30, repeats: true) { [weak self] _ in self?.tick() }
        RunLoop.main.add(timer, forMode: .common)
        self.timer = timer
        tick()
    }

    func resize(_ size: CGSize) {
        canvasSize = size
    }

    func previewNext() {
        storyIndex = storyIndex % Self.stories.count + 1
        previewStart = ProcessInfo.processInfo.systemUptime
        storyTitle = Self.stories[storyIndex - 1].0
    }

    func stop() {
        if musicEnabled { setMusic(false) }
        timer?.invalidate()
        timer = nil
        output.fileHandleForReading.readabilityHandler = nil
        try? input.fileHandleForWriting.close()
        if process.isRunning { process.terminate() }
    }

    func setMusic(_ enabled: Bool) {
        guard #available(macOS 13.0, *), !audioChanging, enabled != musicEnabled else { return }
        audioChanging = true
        audioError = ""
        if enabled {
            let audio = SystemAudio { [weak self] ended in
                guard let self, self.capture === ended else { return }
                self.capture = nil
                self.musicEnabled = false
                self.audioError = "시스템 오디오 연결이 끊겼습니다."
            }
            Task { @MainActor in
                defer { audioChanging = false }
                do {
                    try await audio.start()
                    capture = audio
                    musicEnabled = true
                } catch {
                    audioError = "시스템 설정에서 화면 및 시스템 오디오 녹화 권한을 확인해 주세요."
                }
            }
        } else if let audio = capture as? SystemAudio {
            capture = nil
            musicEnabled = false
            Task { @MainActor in
                await audio.stop()
                audioChanging = false
            }
        }
    }

    private func tick() {
        let now = ProcessInfo.processInfo.systemUptime
        if pending {
            if now - sentAt > 5 { fail("트리 엔진의 응답이 지연되고 있습니다.") }
            return
        }
        guard process.isRunning else { fail("트리 엔진이 종료되었습니다."); return }
        let direction = settings.Direction == "Static" ? 0.0 : (settings.Direction == "Counterclockwise" ? -1.0 : 1.0)
        angle += min(now - lastTick, 0.15) * 0.85 * Double(settings.Speed) / 100 * direction
        lastTick = now
        let progress = now - previewStart
        if !storyTitle.isEmpty && progress >= Self.stories[storyIndex - 1].1 {
            storyTitle = ""
        }
        let size = max(9, canvasSize.height / 57)
        var samples = Data()
        var sampleRate = 0
        if #available(macOS 13.0, *), let audio = capture as? SystemAudio {
            (samples, sampleRate) = audio.drain()
        }
        let request = FrameRequest(
            Columns: max(16, Int(canvasSize.width / (size * 0.62))),
            Rows: max(12, Int(canvasSize.height / (size * 1.18))),
            Elapsed: now - start, Angle: angle, StoryPreview: storyTitle.isEmpty ? 0 : storyIndex,
            StoryElapsed: progress, AudioPCM: samples, AudioRate: sampleRate, AudioEnabled: musicEnabled,
            Equalizer: equalizer, Settings: settings)
        do {
            var data = try JSONEncoder().encode(request)
            data.append(10)
            try input.fileHandleForWriting.write(contentsOf: data)
            pending = true
            sentAt = now
        } catch {
            fail("트리 엔진에 설정을 전달할 수 없습니다.")
        }
    }

    private func receive(_ bytes: Data) {
        guard error == nil else { return }
        guard !bytes.isEmpty else { fail("트리 엔진이 종료되었습니다."); return }
        buffer.append(bytes)
        guard let end = buffer.firstIndex(of: 10) else { return }
        do {
            let next = try JSONDecoder().decode(TreeFrame.self, from: Data(buffer[..<end]))
            buffer.removeSubrange(...end)
            frame = next
            pending = false
            if smokeTest { checkSmokeTest(next) }
        } catch {
            fail("트리 화면을 읽을 수 없습니다.")
        }
    }

    private func checkSmokeTest(_ next: TreeFrame) {
        guard !next.Cells.isEmpty else { fail("렌더링 결과가 비었습니다."); return }
        if smokeStage == 0 {
            guard !next.Greeting.isEmpty else { fail("기본 인사말이 표시되지 않았습니다."); return }
            settings.Couple = CoupleSettings(Enabled: true, FirstName: "지우", SecondName: "하늘", StartDate: "2020-01-01")
            previewNext()
            smokeStage = 1
        } else {
            guard next.FirstName == "지우", next.Counter.hasPrefix("D+"), storyTitle == "산타와 루돌프" else {
                fail("기념일, 이야기 설정 또는 macOS 창 표시 검증이 실패했습니다.")
                return
            }
            guard next.Cells.contains(where: { $0.Glyph == "o" && $0.Color == 0xFF424F }),
                  let preview = NSApp.windows.first(where: { $0.isVisible && $0.contentView != nil }) else { return }
            preview.displayIfNeeded()
            stop()
            print("macOS 창 표시, 트리 렌더링, 기본 인사말, 커플 기념일, 이야기 미리보기 검증 통과")
            exit(EXIT_SUCCESS)
        }
    }

    private func fail(_ message: String) {
        error = message
        stop()
        if smokeTest {
            FileHandle.standardError.write(Data((message + "\n").utf8))
            exit(EXIT_FAILURE)
        }
    }
}
