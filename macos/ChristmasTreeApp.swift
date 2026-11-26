import AppKit
import SwiftUI

@main
struct ChristmasTreeApp: App {
    @NSApplicationDelegateAdaptor(AppDelegate.self) private var appDelegate
    @StateObject private var renderer = TreeProcess()

    var body: some Scene {
        WindowGroup("Developer Christmas Tree") {
            ChristmasTreeWindow(renderer: renderer)
        }
        .commands {
            CommandGroup(replacing: .newItem) {}
            CommandMenu("화면") {
                Button("전체 화면 전환") { NSApp.keyWindow?.toggleFullScreen(nil) }
                    .keyboardShortcut("f", modifiers: [.command, .control])
            }
        }
    }
}

private final class AppDelegate: NSObject, NSApplicationDelegate {
    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool { true }
}

private struct ChristmasTreeWindow: View {
    @ObservedObject var renderer: TreeProcess
    @State private var fullscreen = false

    var body: some View {
        HStack(spacing: 0) {
            VStack(alignment: .leading, spacing: 12) {
                if !fullscreen {
                    Text("실시간 미리보기").font(.headline)
                }
                TreePreview(frame: renderer.frame, error: renderer.error) { renderer.resize($0) }
            }
            .padding(fullscreen ? 0 : 18)
            if !fullscreen {
                Divider()
                TreeControls(renderer: renderer)
                    .frame(width: 306)
            }
        }
        .frame(minWidth: 780, minHeight: 570)
        .onAppear { renderer.begin() }
        .onReceive(NotificationCenter.default.publisher(for: NSApplication.willTerminateNotification)) { _ in
            renderer.stop()
        }
        .onReceive(NotificationCenter.default.publisher(for: NSWindow.didEnterFullScreenNotification)) { _ in
            fullscreen = true
        }
        .onReceive(NotificationCenter.default.publisher(for: NSWindow.didExitFullScreenNotification)) { _ in
            fullscreen = false
        }
    }
}

private struct TreePreview: View {
    let frame: TreeFrame?
    let error: String?
    let resize: (CGSize) -> Void

    var body: some View {
        VStack(spacing: 0) {
            GeometryReader { viewport in
                AppKitTree(frame: frame)
                .onAppear { resize(viewport.size) }
                .onChange(of: viewport.size, perform: resize)
                .overlay {
                    if let error {
                        Text(error).foregroundColor(.white).multilineTextAlignment(.center).padding()
                    } else if frame == nil {
                        ProgressView("트리 불러오는 중").foregroundColor(.white)
                    }
                }
            }
            if let frame {
                if let bars = frame.AudioBars {
                    HStack(alignment: .bottom, spacing: 2) {
                        ForEach(bars.indices, id: \.self) { i in
                            Rectangle().fill(Color(hue: Double(i) / Double(bars.count), saturation: 0.65, brightness: 0.85))
                                .frame(height: 2 + 34 * CGFloat(bars[i])).frame(maxWidth: .infinity)
                        }
                    }
                    .frame(height: 40, alignment: .bottom).padding(.horizontal, 18)
                }
                if !frame.FirstName.isEmpty {
                    (
                        Text(frame.FirstName + " ").foregroundColor(.white) +
                        Text("♥").foregroundColor(rgb(0xFF454F)) +
                        Text(" " + frame.SecondName + "  " + frame.Counter).foregroundColor(.white)
                    )
                    .lineLimit(1).minimumScaleFactor(0.7).padding(12)
                } else if !frame.Greeting.isEmpty {
                    VStack(spacing: 3) {
                        Text("<<<  HAPPY CODING  >>>").foregroundColor(rgb(0xF9D98A))
                        Text(frame.Greeting).foregroundColor(rgb(0x82E8EB))
                    }
                    .font(.system(.callout, design: .monospaced))
                    .lineLimit(1).minimumScaleFactor(0.65).padding(10)
                }
            }
        }
        .background(rgb(frame?.Background ?? 0x0D0D0D))
        .clipShape(RoundedRectangle(cornerRadius: 12))
    }

    private func rgb(_ value: UInt32) -> Color {
        Color(red: Double((value >> 16) & 255) / 255,
              green: Double((value >> 8) & 255) / 255, blue: Double(value & 255) / 255)
    }
}

private struct AppKitTree: NSViewRepresentable {
    let frame: TreeFrame?

    func makeNSView(context: Context) -> TreeDrawingView { TreeDrawingView() }

    func updateNSView(_ view: TreeDrawingView, context: Context) {
        view.frameData = frame
        view.needsDisplay = true
    }
}

private final class TreeDrawingView: NSView {
    var frameData: TreeFrame?
    override var isFlipped: Bool { true }

    override func draw(_ dirtyRect: NSRect) {
        guard let frameData else { return }
        let width = bounds.width / CGFloat(frameData.Columns)
        let height = bounds.height / CGFloat(frameData.Rows)
        let font = NSFont.monospacedSystemFont(ofSize: min(width / 0.61, height / 1.15), weight: .regular)
        var colors: [UInt32: NSColor] = [:]
        for cell in frameData.Cells {
            let position = NSPoint(x: (CGFloat(cell.X) + 0.5) * width, y: (CGFloat(cell.Y) + 0.5) * height)
            let color = colors[cell.Color] ?? NSColor(
                srgbRed: CGFloat((cell.Color >> 16) & 255) / 255,
                green: CGFloat((cell.Color >> 8) & 255) / 255, blue: CGFloat(cell.Color & 255) / 255, alpha: 1)
            colors[cell.Color] = color
            if cell.Glow {
                let glimmer = NSRect(x: position.x - width * 0.75, y: position.y - height * 0.38,
                                     width: width * 1.5, height: height * 0.76)
                color.withAlphaComponent(0.14).setFill()
                NSBezierPath(ovalIn: glimmer).fill()
            }
            let glyph = cell.Glyph as NSString
            let attributes: [NSAttributedString.Key: Any] = [.font: font, .foregroundColor: color]
            let size = glyph.size(withAttributes: attributes)
            glyph.draw(at: NSPoint(x: position.x - size.width / 2, y: position.y - size.height / 2),
                       withAttributes: attributes)
        }
    }
}

private struct TreeControls: View {
    @ObservedObject var renderer: TreeProcess
    private let themes = [
        ("무지개", "Rainbow RGB"), ("클래식 크리스마스", "Classic Christmas"), ("아이스 블루", "Ice Blue"),
        ("사이버펑크", "Cyberpunk"), ("골드 & 화이트", "Gold & White"), ("매트릭스 그린", "Matrix Green"),
    ]
    private static let dateFormat: DateFormatter = {
        let format = DateFormatter()
        format.locale = Locale(identifier: "en_US_POSIX")
        format.calendar = Calendar(identifier: .gregorian)
        format.dateFormat = "yyyy-MM-dd"
        return format
    }()

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 15) {
                HStack {
                    Text("트리 설정").font(.headline)
                    Spacer()
                    Text("v" + (Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String ?? ""))
                        .foregroundColor(.secondary).font(.caption)
                }
                Picker("색상", selection: $renderer.settings.Theme) {
                    ForEach(themes.indices, id: \.self) { i in Text(themes[i].0).tag(themes[i].1) }
                }
                Picker("회전 방향", selection: $renderer.settings.Direction) {
                    Text("시계 방향").tag("Clockwise")
                    Text("반시계 방향").tag("Counterclockwise")
                    Text("정지").tag("Static")
                }
                slider("회전 속도", $renderer.settings.Speed, range: 0...400)
                slider("조명 변화 속도", $renderer.settings.LightSpeed, range: 0...400)
                slider("트리 크기", $renderer.settings.Size, range: 45...100)
                Divider()
                if #available(macOS 13.0, *) {
                    Toggle("시스템 음악 반응", isOn: Binding(get: { renderer.musicEnabled }, set: renderer.setMusic))
                        .disabled(renderer.audioChanging)
                    if renderer.musicEnabled {
                        Picker("이퀄라이저", selection: $renderer.equalizer) {
                            Text("트리 둘레").tag("ring")
                            Text("하단 막대").tag("bars")
                            Text("끄기").tag("off")
                        }
                        Text(renderer.frame?.AudioActive == true ? "시스템 오디오 감지됨" : "다른 앱에서 음악을 재생해 주세요.")
                            .font(.caption).foregroundColor(.secondary)
                    }
                    if !renderer.audioError.isEmpty {
                        Text(renderer.audioError).font(.caption).foregroundColor(.secondary)
                    }
                } else {
                    Text("시스템 음악 반응은 macOS 13 이상에서 사용할 수 있습니다.")
                        .font(.caption).foregroundColor(.secondary)
                }
                Divider()
                Toggle("눈 내리기", isOn: $renderer.settings.Snow)
                Toggle("코드 장식", isOn: $renderer.settings.Ornaments)
                Toggle("코드 비", isOn: $renderer.settings.Matrix)
                Toggle("빛 번짐", isOn: $renderer.settings.Bloom)
                Toggle("크리스마스 D-Day", isOn: $renderer.settings.Countdown)
                Toggle("성탄 이야기", isOn: $renderer.settings.Stories)
                HStack {
                    Button("이야기 미리보기") { renderer.previewNext() }
                    Text(renderer.storyTitle).font(.caption).foregroundColor(.secondary)
                }
                Divider()
                Toggle("인사말 표시", isOn: $renderer.settings.Messages)
                TextField("나만의 문구", text: $renderer.settings.Message)
                Toggle("커플 기념일 표시", isOn: $renderer.settings.Couple.Enabled)
                if renderer.settings.Couple.Enabled {
                    HStack {
                        TextField("첫 번째 이름", text: $renderer.settings.Couple.FirstName)
                        TextField("두 번째 이름", text: $renderer.settings.Couple.SecondName)
                    }
                    DatePicker("시작일", selection: Binding(
                        get: { Self.dateFormat.date(from: renderer.settings.Couple.StartDate) ?? Date() },
                        set: { renderer.settings.Couple.StartDate = Self.dateFormat.string(from: $0) }
                    ), in: ...Date(), displayedComponents: .date)
                    if renderer.settings.Couple.StartDate.isEmpty {
                        Button("오늘로 설정") {
                            renderer.settings.Couple.StartDate = Self.dateFormat.string(from: Date())
                        }
                    }
                    if let error = renderer.frame?.CoupleError, !error.isEmpty {
                        Text(error).font(.caption).foregroundColor(.secondary)
                    }
                }
                Divider()
                Button("전체 화면") { NSApp.keyWindow?.toggleFullScreen(nil) }
            }
            .padding(18)
        }
    }

    private func slider(_ label: String, _ value: Binding<Int>, range: ClosedRange<Double>) -> some View {
        HStack {
            Slider(value: Binding(get: { Double(value.wrappedValue) },
                                  set: { value.wrappedValue = Int($0) }), in: range, step: 1) {
                Text(label)
            }
            Text("\(value.wrappedValue)%").monospacedDigit().frame(width: 44, alignment: .trailing)
        }
    }
}
