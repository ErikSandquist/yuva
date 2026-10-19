import SwiftUI
#if os(iOS)
import SafariServices
#endif

struct YuvaStatusLine: View {
    let status: YuvaInboxStatus?
    private let dismissals = StatusDismissals.shared
    @State private var browsing: BrowserTarget?
    #if os(macOS)
    @Environment(\.openURL) private var openURL
    #endif

    var body: some View {
        Group {
            if let notice = dismissals.notice(for: status) {
                row(notice)
                    .transition(.move(edge: .top).combined(with: .opacity))
            }
        }
        .animation(.snappy, value: dismissals.notice(for: status))
        .onChange(of: status, initial: true) { _, status in dismissals.prune(keeping: status) }
        #if os(iOS)
        .fullScreenCover(item: $browsing) { SafariView(url: $0.url).ignoresSafeArea() }
        #endif
    }

    private func row(_ notice: StatusNotice) -> some View {
        let lead = notice.lead
        let tint = StatusStyle.tint(lead)
        return HStack(spacing: 4) {
            Button {
                open(lead.url.isEmpty ? status?.url : lead.url)
            } label: {
                HStack(spacing: 12) {
                    Image(systemName: StatusStyle.symbol(lead))
                        .font(.title3)
                        .foregroundStyle(tint)
                        .frame(width: 28)
                        .accessibilityHidden(true)
                    VStack(alignment: .leading, spacing: 2) {
                        Text(lead.title).font(.subheadline.weight(.semibold)).foregroundStyle(.primary)
                        Text(subtitle(notice)).font(.caption).foregroundStyle(.secondary)
                    }
                    .multilineTextAlignment(.leading)
                    .frame(maxWidth: .infinity, alignment: .leading)
                }
                .contentShape(Rectangle())
            }
            .buttonStyle(.plain)
            .accessibilityElement(children: .ignore)
            .accessibilityLabel("\(lead.title), \(subtitle(notice))")
            .accessibilityHint(L.t("status.openHint"))
            .accessibilityAddTraits(.isLink)
            .accessibilityIdentifier("yuva.status.open")
            Button {
                dismissals.dismiss(notice)
            } label: {
                Image(systemName: "xmark").font(.footnote.weight(.semibold)).foregroundStyle(.secondary)
                    .frame(width: 32, height: 32)
                    .contentShape(Rectangle())
            }
            .buttonStyle(.plain)
            .accessibilityLabel(L.t("status.dismiss"))
            .accessibilityIdentifier("yuva.status.dismiss")
        }
        .padding(.leading, 12).padding(.trailing, 6).padding(.vertical, 10)
        .background(tint.opacity(0.14), in: RoundedRectangle(cornerRadius: 16, style: .continuous))
        .background(.regularMaterial, in: RoundedRectangle(cornerRadius: 16, style: .continuous))
        .padding(.horizontal, 16).padding(.vertical, 6)
    }

    private func subtitle(_ notice: StatusNotice) -> String {
        var parts = [StatusStyle.kindLabel(notice.lead)]
        if let state = StatusStyle.stateLabel(notice.lead.state) { parts.append(state) }
        if notice.more > 0 { parts.append(L.f("status.more", notice.more)) }
        return parts.joined(separator: " · ")
    }

    private func open(_ text: String?) {
        guard let text, let url = URL(string: text), ["http", "https"].contains(url.scheme?.lowercased()) else { return }
        #if os(iOS)
        browsing = BrowserTarget(url: url)
        #else
        openURL(url)
        #endif
    }
}

struct BrowserTarget: Identifiable {
    let url: URL
    var id: URL { url }
}

enum StatusStyle {
    static func tint(_ incident: YuvaStatusIncident) -> Color {
        if incident.kind == .maintenance { return .blue }
        switch incident.impact {
        case .critical: return .red
        case .major: return .orange
        case .minor: return .yellow
        case .none: return .secondary
        }
    }

    static func symbol(_ incident: YuvaStatusIncident) -> String {
        if incident.kind == .maintenance { return "wrench.and.screwdriver.fill" }
        switch incident.impact {
        case .critical, .major: return "exclamationmark.triangle.fill"
        case .minor: return "exclamationmark.circle.fill"
        case .none: return "info.circle.fill"
        }
    }

    static func kindLabel(_ incident: YuvaStatusIncident) -> String {
        if incident.kind == .maintenance { return L.t("status.kind.maintenance") }
        switch incident.impact {
        case .critical: return L.t("status.kind.critical")
        case .major: return L.t("status.kind.major")
        case .minor: return L.t("status.kind.minor")
        case .none: return L.t("status.kind.incident")
        }
    }

    static func stateLabel(_ state: YuvaStatusState) -> String? {
        switch state {
        case .investigating: L.t("status.state.investigating")
        case .identified: L.t("status.state.identified")
        case .monitoring: L.t("status.state.monitoring")
        case .resolved: L.t("status.state.resolved")
        case .scheduled: L.t("status.state.scheduled")
        case .inProgress: L.t("status.state.inProgress")
        case .completed: L.t("status.state.completed")
        case .unknown: nil
        }
    }
}

#if os(iOS)
struct SafariView: UIViewControllerRepresentable {
    let url: URL

    func makeUIViewController(context: Context) -> SFSafariViewController { SFSafariViewController(url: url) }

    func updateUIViewController(_ controller: SFSafariViewController, context: Context) {}
}
#endif
