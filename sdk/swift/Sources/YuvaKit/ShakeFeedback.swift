import SwiftUI

@MainActor
enum OpenScreens {
    private(set) static var open = 0

    static func enter() { open += 1 }
    static func leave() { open = max(0, open - 1) }
}

extension View {
    func countsAsOpen() -> some View {
        onAppear { OpenScreens.enter() }.onDisappear { OpenScreens.leave() }
    }
}

#if os(iOS)
import ObjectiveC
import UIKit

extension Yuva {
    @MainActor
    public static func enableShakeToFeedback(
        client: YuvaClient, screen: @escaping @MainActor () -> String? = { nil },
        isEnabled: @escaping @MainActor () -> Bool = { true }
    ) {
        ShakeFeedback.shared.enable(client: client, screen: screen, isEnabled: isEnabled)
    }

    @MainActor
    public static func disableShakeToFeedback() {
        ShakeFeedback.shared.disable()
    }
}

@MainActor
final class ShakeFeedback {
    static let shared = ShakeFeedback()

    private var client: YuvaClient?
    private var screen: @MainActor () -> String? = { nil }
    private var isEnabled: @MainActor () -> Bool = { true }
    private var busy = false
    private var hooked = false

    func enable(
        client: YuvaClient, screen: @escaping @MainActor () -> String?, isEnabled: @escaping @MainActor () -> Bool
    ) {
        self.client = client
        self.screen = screen
        self.isEnabled = isEnabled
        hook()
    }

    func disable() {
        client = nil
    }

    func finish() {
        busy = false
    }

    func shaken(in window: UIWindow) {
        guard let client, !busy, isEnabled(), OpenScreens.open == 0, window.windowLevel == .normal,
              !Self.isEditingText(in: window),
              let top = Self.topController(from: window.rootViewController),
              !(top is UIAlertController), !(top is YuvaFeedbackHost),
              let screenshot = Self.capture(window)
        else { return }
        let name = screen() ?? Self.name(of: top)
        busy = true
        let alert = UIAlertController(
            title: L.t("feedback.shake.title"), message: L.t("feedback.shake.message"), preferredStyle: .alert)
        alert.addAction(
            UIAlertAction(title: L.t("feedback.cancel"), style: .cancel) { [weak self] _ in
                MainActor.assumeIsolated { self?.finish() }
            })
        let send = UIAlertAction(title: L.t("feedback.send"), style: .default) { [weak self] _ in
            MainActor.assumeIsolated { self?.present(client: client, screenshot: screenshot, screen: name, over: top) }
        }
        alert.addAction(send)
        alert.preferredAction = send
        top.present(alert, animated: true)
    }

    private func present(client: YuvaClient, screenshot: YuvaUpload, screen: String, over top: UIViewController) {
        let box = HostBox()
        let host = YuvaFeedbackHost(
            rootView: YuvaFeedbackView(client: client, screenshot: screenshot, screen: screen) {
                box.host?.dismiss(animated: true)
            })
        box.host = host
        top.present(host, animated: true)
    }

    private static func capture(_ window: UIWindow) -> YuvaUpload? {
        let renderer = UIGraphicsImageRenderer(bounds: window.bounds)
        let data = renderer.jpegData(withCompressionQuality: 0.8) { _ in
            window.drawHierarchy(in: window.bounds, afterScreenUpdates: false)
        }
        return data.isEmpty ? nil : YuvaUpload(filename: "screenshot.jpg", contentType: "image/jpeg", data: data)
    }

    private static func isEditingText(in view: UIView) -> Bool {
        if view.isFirstResponder, view is UITextInput { return true }
        return view.subviews.contains(where: isEditingText)
    }

    private static func topController(from root: UIViewController?) -> UIViewController? {
        var current = root
        while let controller = current {
            if let presented = controller.presentedViewController, !presented.isBeingDismissed {
                current = presented
            } else if let navigation = controller as? UINavigationController, let top = navigation.topViewController {
                current = top
            } else if let tabs = controller as? UITabBarController, let selected = tabs.selectedViewController {
                current = selected
            } else {
                return controller
            }
        }
        return nil
    }

    private static func name(of controller: UIViewController) -> String {
        String(String(describing: type(of: controller)).prefix { $0 != "<" })
    }

    private func hook() {
        guard !hooked else { return }
        hooked = true
        let original = #selector(UIResponder.motionEnded(_:with:))
        let replacement = #selector(UIWindow.yuvaMotionEnded(_:with:))
        guard let replacementMethod = class_getInstanceMethod(UIWindow.self, replacement) else { return }
        if class_addMethod(
            UIWindow.self, original, method_getImplementation(replacementMethod),
            method_getTypeEncoding(replacementMethod))
        {
            guard let inherited = class_getInstanceMethod(UIResponder.self, original) else { return }
            method_setImplementation(replacementMethod, method_getImplementation(inherited))
        } else if let originalMethod = class_getInstanceMethod(UIWindow.self, original) {
            method_exchangeImplementations(originalMethod, replacementMethod)
        }
    }
}

private final class HostBox {
    weak var host: UIViewController?
}

final class YuvaFeedbackHost: UIHostingController<YuvaFeedbackView> {
    override func viewDidDisappear(_ animated: Bool) {
        super.viewDidDisappear(animated)
        if isBeingDismissed { ShakeFeedback.shared.finish() }
    }
}

extension UIWindow {
    @objc func yuvaMotionEnded(_ motion: UIEvent.EventSubtype, with event: UIEvent?) {
        yuvaMotionEnded(motion, with: event)
        if motion == .motionShake { ShakeFeedback.shared.shaken(in: self) }
    }
}
#endif
