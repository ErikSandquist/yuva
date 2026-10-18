import SwiftUI
import UniformTypeIdentifiers
#if canImport(UIKit)
import UIKit
typealias PlatformImage = UIImage
#else
import AppKit
typealias PlatformImage = NSImage
#endif

enum L {
    static func t(_ key: String) -> String {
        String(localized: String.LocalizationValue(key), bundle: .module)
    }

    static func f(_ key: String, _ args: CVarArg...) -> String {
        String(format: t(key), arguments: args)
    }

    static func message(for error: Error) -> String {
        if error is URLError { return t("error.network") }
        guard let error = error as? YuvaError else { return t("error.generic") }
        switch error.code {
        case "attachment_too_large": return t("error.attachmentTooLarge")
        case "attachment_type_not_allowed": return t("error.attachmentType")
        case "attachment_type_mismatch": return t("error.attachmentMismatch")
        default: break
        }
        switch error.status {
        case 413: return t("error.attachmentTooLarge")
        case 415: return t("error.attachmentType")
        default: return t("error.generic")
        }
    }
}

extension Image {
    init(platformImage: PlatformImage) {
        #if canImport(UIKit)
        self.init(uiImage: platformImage)
        #else
        self.init(nsImage: platformImage)
        #endif
    }
}

extension Color {
    init?(yuvaHex: String?) {
        guard let hex = yuvaHex, hex.count == 7, hex.hasPrefix("#"), let value = UInt32(hex.dropFirst(), radix: 16) else {
            return nil
        }
        self.init(
            red: Double((value >> 16) & 0xFF) / 255, green: Double((value >> 8) & 0xFF) / 255,
            blue: Double(value & 0xFF) / 255)
    }
}

enum ImageUpload {
    static func make(from data: Data, name: String = "image") -> YuvaUpload? {
        guard let image = PlatformImage(data: data) else { return nil }
        #if canImport(UIKit)
        guard let jpeg = image.jpegData(compressionQuality: 0.8) else { return nil }
        #else
        guard let tiff = image.tiffRepresentation, let rep = NSBitmapImageRep(data: tiff),
              let jpeg = rep.representation(using: .jpeg, properties: [.compressionFactor: 0.8])
        else { return nil }
        #endif
        return YuvaUpload(filename: name + ".jpg", contentType: "image/jpeg", data: jpeg)
    }
}

struct AvatarView: View {
    let initials: String
    var size: CGFloat = 28
    var imageURL: String?

    var body: some View {
        Text(initials.isEmpty ? "?" : initials)
            .font(.system(size: size * 0.4, weight: .semibold))
            .foregroundStyle(.white)
            .frame(width: size, height: size)
            .background(Circle().fill(Color.accentColor.gradient))
            .overlay {
                if let url = imageURL.flatMap(URL.init(string:)), url.scheme?.hasPrefix("http") == true {
                    AsyncImage(url: url) { phase in
                        if let image = phase.image { image.resizable().scaledToFill() }
                    }
                    .clipShape(Circle())
                }
            }
            .accessibilityHidden(true)
    }
}

struct CategoryIcon: View {
    let category: YuvaFeedbackCategory?
    var size: CGFloat = 44

    var body: some View {
        Image(systemName: category?.symbol ?? "bubble.left.fill")
            .font(.system(size: size * 0.42, weight: .semibold))
            .foregroundStyle(.white)
            .frame(width: size, height: size)
            .background(Circle().fill((category?.tint ?? .gray).gradient))
            .accessibilityHidden(true)
    }
}

extension YuvaFeedbackCategory {
    var symbol: String {
        switch self {
        case .bug: "ladybug.fill"
        case .idea: "lightbulb.fill"
        case .praise: "heart.fill"
        case .other: "ellipsis.bubble.fill"
        }
    }

    var tint: Color {
        switch self {
        case .bug: .red
        case .idea: .orange
        case .praise: .pink
        case .other: .gray
        }
    }
}

struct FeedbackTag: View {
    let text: String
    let tint: Color

    var body: some View {
        Text(text)
            .font(.caption2.weight(.semibold))
            .foregroundStyle(tint)
            .padding(.horizontal, 8).padding(.vertical, 3)
            .background(Capsule().fill(tint.opacity(0.15)))
    }
}

extension YuvaFeedbackState {
    var label: String { L.t("feedback.state." + rawValue) }

    var tint: Color {
        switch self {
        case .sent: .secondary
        case .answered: .accentColor
        case .resolved, .done: .green
        case .planned: Color(red: 0.35, green: 0.40, blue: 0.85)
        case .inProgress: .orange
        case .wontDo: .gray
        }
    }
}

enum ShortTime {
    static func string(_ date: Date, now: Date = Date(), calendar: Calendar = .current) -> String {
        if calendar.isDate(date, inSameDayAs: now) {
            return date.formatted(date: .omitted, time: .shortened)
        }
        if calendar.isDate(date, inSameDayAs: calendar.date(byAdding: .day, value: -1, to: now) ?? now) {
            return relativeDay(date, style: .short)
        }
        let start = calendar.startOfDay(for: now)
        if let days = calendar.dateComponents([.day], from: calendar.startOfDay(for: date), to: start).day,
           days > 0, days < 7 {
            return date.formatted(.dateTime.weekday(.wide))
        }
        return date.formatted(date: .numeric, time: .omitted)
    }

    static func daySeparator(_ date: Date) -> String {
        relativeDay(date, style: .medium)
    }

    static func clock(_ date: Date) -> String {
        date.formatted(date: .omitted, time: .shortened)
    }

    private static func relativeDay(_ date: Date, style: DateFormatter.Style) -> String {
        let formatter = DateFormatter()
        formatter.dateStyle = style
        formatter.timeStyle = .none
        formatter.doesRelativeDateFormatting = true
        return formatter.string(from: date)
    }
}

@MainActor
final class AttachmentCache {
    static let shared = AttachmentCache()
    private var images: [String: PlatformImage] = [:]

    func image(_ id: String, client: YuvaClient) async -> PlatformImage? {
        if let image = images[id] { return image }
        guard let data = try? await client.attachment(id: id), let image = PlatformImage(data: data) else { return nil }
        images[id] = image
        return image
    }
}

struct AttachmentImageView: View {
    let client: YuvaClient
    let attachment: YuvaAttachment
    @State private var image: PlatformImage?

    var body: some View {
        Group {
            if let image {
                Image(platformImage: image).resizable().scaledToFit()
            } else {
                RoundedRectangle(cornerRadius: 12).fill(.quaternary).frame(width: 180, height: 140)
                    .overlay(ProgressView())
            }
        }
        .frame(maxWidth: 220, maxHeight: 260)
        .clipShape(RoundedRectangle(cornerRadius: 12))
        .task(id: attachment.id) {
            for attempt in 1...3 {
                image = await AttachmentCache.shared.image(attachment.id, client: client)
                if image != nil || Task.isCancelled { return }
                try? await Task.sleep(for: .seconds(2 * attempt))
            }
        }
    }
}

struct AttachmentFileView: View {
    let client: YuvaClient
    let attachment: YuvaAttachment
    @State private var url: URL?
    @State private var loading = false

    var body: some View {
        Group {
            if let url {
                ShareLink(item: url) { label(systemImage: "square.and.arrow.up") }
            } else {
                Button {
                    Task { await download() }
                } label: {
                    label(systemImage: loading ? "hourglass" : "paperclip")
                }
                .buttonStyle(.plain)
            }
        }
    }

    private func label(systemImage: String) -> some View {
        HStack(spacing: 6) {
            Image(systemName: systemImage)
            Text(attachment.filename).lineLimit(1)
            Text(ByteCountFormatter.string(fromByteCount: attachment.size, countStyle: .file))
                .foregroundStyle(.secondary)
        }
        .font(.footnote)
        .padding(.horizontal, 10)
        .padding(.vertical, 6)
        .background(Capsule().fill(.quaternary))
    }

    private func download() async {
        loading = true
        defer { loading = false }
        guard let data = try? await client.attachment(id: attachment.id) else { return }
        let directory = FileManager.default.temporaryDirectory.appending(path: attachment.id)
        try? FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
        let file = directory.appending(path: attachment.filename)
        if (try? data.write(to: file)) != nil { url = file }
    }
}
