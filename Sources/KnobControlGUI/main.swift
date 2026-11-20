import AppKit
import SwiftUI
import IOKit.hid
import AudioToolbox

private let vendorID = 0x05ac
private let productID = 0x022c

private enum KnobInput: String, CaseIterable, Identifiable, Codable {
    case rotateLeft = "rotate_left"
    case rotateRight = "rotate_right"
    case press = "press"
    case pressRotateLeft = "press_rotate_left"
    case pressRotateRight = "press_rotate_right"

    var id: String { rawValue }

    var title: String {
        switch self {
        case .rotateLeft: return "Rotate left"
        case .rotateRight: return "Rotate right"
        case .press: return "Press"
        case .pressRotateLeft: return "Hold + rotate left"
        case .pressRotateRight: return "Hold + rotate right"
        }
    }
}

private enum ModifierMode: String, CaseIterable, Identifiable, Codable {
    case none
    case command
    case shift
    case option
    case control
    case commandShift = "command_shift"
    case commandOption = "command_option"
    case commandControl = "command_control"
    case shiftOption = "shift_option"
    case shiftControl = "shift_control"
    case optionControl = "option_control"
    case commandShiftOption = "command_shift_option"
    case commandShiftControl = "command_shift_control"
    case commandOptionControl = "command_option_control"
    case shiftOptionControl = "shift_option_control"
    case commandShiftOptionControl = "command_shift_option_control"

    var id: String { rawValue }

    var title: String {
        switch self {
        case .none: return "No modifier"
        case .command: return "⌘ Command"
        case .shift: return "⇧ Shift"
        case .option: return "⌥ Option"
        case .control: return "⌃ Control"
        case .commandShift: return "⌘⇧ Command + Shift"
        case .commandOption: return "⌘⌥ Command + Option"
        case .commandControl: return "⌘⌃ Command + Control"
        case .shiftOption: return "⇧⌥ Shift + Option"
        case .shiftControl: return "⇧⌃ Shift + Control"
        case .optionControl: return "⌥⌃ Option + Control"
        case .commandShiftOption: return "⌘⇧⌥ Command + Shift + Option"
        case .commandShiftControl: return "⌘⇧⌃ Command + Shift + Control"
        case .commandOptionControl: return "⌘⌥⌃ Command + Option + Control"
        case .shiftOptionControl: return "⇧⌥⌃ Shift + Option + Control"
        case .commandShiftOptionControl: return "⌘⇧⌥⌃ All modifiers"
        }
    }

    var compactTitle: String {
        switch self {
        case .none: return ""
        case .command: return "⌘"
        case .shift: return "⇧"
        case .option: return "⌥"
        case .control: return "⌃"
        case .commandShift: return "⌘⇧"
        case .commandOption: return "⌘⌥"
        case .commandControl: return "⌘⌃"
        case .shiftOption: return "⇧⌥"
        case .shiftControl: return "⇧⌃"
        case .optionControl: return "⌥⌃"
        case .commandShiftOption: return "⌘⇧⌥"
        case .commandShiftControl: return "⌘⇧⌃"
        case .commandOptionControl: return "⌘⌥⌃"
        case .shiftOptionControl: return "⇧⌥⌃"
        case .commandShiftOptionControl: return "⌘⇧⌥⌃"
        }
    }

    func mappingKey(for input: KnobInput) -> String {
        self == .none ? input.rawValue : "\(rawValue)+\(input.rawValue)"
    }

    func gestureTitle(for input: KnobInput) -> String {
        let base = "\(gestureSymbol(input)) \(input.title)"
        guard self != .none else { return base }
        return "\(compactTitle) + \(base)"
    }

    static func current() -> ModifierMode {
        let flags = CGEventSource.flagsState(.combinedSessionState)
        var parts: [String] = []
        if flags.contains(.maskCommand) { parts.append("command") }
        if flags.contains(.maskShift) { parts.append("shift") }
        if flags.contains(.maskAlternate) { parts.append("option") }
        if flags.contains(.maskControl) { parts.append("control") }
        guard !parts.isEmpty else { return .none }
        return allCases.first { $0.rawValue == parts.joined(separator: "_") } ?? .none
    }
}

private enum ActionType: String, CaseIterable, Identifiable, Codable {
    case shortcut
    case key
    case mouse
    case scroll
    case shell
    case noop

    var id: String { rawValue }

    var title: String {
        switch self {
        case .shortcut: return "Shortcut"
        case .key: return "Key"
        case .mouse: return "Mouse"
        case .scroll: return "Scroll"
        case .shell: return "Shell"
        case .noop: return "No action"
        }
    }
}

private enum ActionTemplate: String, CaseIterable, Identifiable {
    case browser
    case coding
    case media
    case design
    case editing

    var id: String { rawValue }

    var title: String {
        switch self {
        case .browser: return "Browser"
        case .coding: return "Code"
        case .media: return "Media"
        case .design: return "Design"
        case .editing: return "Video"
        }
    }

    var subtitle: String {
        switch self {
        case .browser: return "Tabs, reload, back/forward"
        case .coding: return "Editor switching, command palette, problem navigation"
        case .media: return "Volume, play/pause, previous/next track"
        case .design: return "Brush size, hardness, brush tool"
        case .editing: return "Frame step, play/pause, big step"
        }
    }
}

private struct MappingConfig: Codable {
    var global: [String: ActionConfig]
    var apps: [String: [String: ActionConfig]]
}

private struct ActionConfig: Codable, Equatable {
    var type: String
    var description: String?
    var keys: [String]?
    var button: String?
    var clicks: Int?
    var dx: Int?
    var dy: Int?
    var command: String?

    static func empty() -> ActionConfig {
        ActionConfig(type: "noop", description: "", keys: [], button: "left", clicks: 1, dx: 0, dy: 0, command: "")
    }

    var isConfiguredAction: Bool {
        switch type {
        case "shortcut", "key":
            return !(keys ?? []).isEmpty
        case "mouse":
            return (clicks ?? 0) > 0
        case "scroll":
            return (dx ?? 0) != 0 || (dy ?? 0) != 0
        case "shell":
            return command?.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty == false
        default:
            return false
        }
    }
}

private struct AppInfo: Identifiable, Hashable {
    let bundleID: String
    let name: String
    let path: String
    let isRunning: Bool

    var id: String { bundleID }
}

private enum ProfileID: Hashable {
    case global
    case app(String)
}

private struct AppContext {
    let name: String
    let bundleID: String
}

private struct ActionResolution {
    let action: ActionConfig?
    let source: String
}

private struct PendingPressAction {
    let date: Date
    let app: AppContext
    let resolution: ActionResolution
}

@MainActor
private final class AppCatalog: ObservableObject {
    @Published var apps: [AppInfo] = []
    @Published var query = ""
    @Published var status = "Not scanned"

    var filteredApps: [AppInfo] {
        let trimmed = query.trimmingCharacters(in: .whitespacesAndNewlines).lowercased()
        guard !trimmed.isEmpty else {
            return apps
        }

        return apps.filter {
            $0.name.lowercased().contains(trimmed)
                || $0.bundleID.lowercased().contains(trimmed)
                || $0.path.lowercased().contains(trimmed)
        }
    }

    func app(bundleID: String) -> AppInfo? {
        apps.first { $0.bundleID == bundleID }
            ?? appInfoFromBundleID(bundleID)
    }

    func refresh() {
        let running = runningApps()
        var byBundleID = running

        for url in installedAppURLs() {
            guard let bundle = Bundle(url: url),
                  let bundleID = bundle.bundleIdentifier,
                  isVisibleApplication(bundle: bundle) else {
                continue
            }

            if byBundleID[bundleID] == nil {
                byBundleID[bundleID] = AppInfo(
                    bundleID: bundleID,
                    name: appName(bundle: bundle, url: url),
                    path: url.path,
                    isRunning: false
                )
            }
        }

        apps = byBundleID.values.sorted {
            if $0.isRunning != $1.isRunning {
                return $0.isRunning && !$1.isRunning
            }
            return $0.name.localizedCaseInsensitiveCompare($1.name) == .orderedAscending
        }
        status = "Found \(apps.count) apps"
    }

    private func runningApps() -> [String: AppInfo] {
        var result: [String: AppInfo] = [:]
        for app in NSWorkspace.shared.runningApplications {
            guard app.activationPolicy == .regular,
                  let bundleID = app.bundleIdentifier else {
                continue
            }

            result[bundleID] = AppInfo(
                bundleID: bundleID,
                name: app.localizedName ?? bundleID,
                path: app.bundleURL?.path ?? "",
                isRunning: true
            )
        }
        return result
    }

    private func installedAppURLs() -> [URL] {
        let fm = FileManager.default
        var roots = [
            URL(fileURLWithPath: "/Applications"),
            URL(fileURLWithPath: "/System/Applications"),
            URL(fileURLWithPath: NSHomeDirectory()).appendingPathComponent("Applications")
        ]

        roots.append(contentsOf: mountedApplicationRoots())

        let keys: [URLResourceKey] = [.isDirectoryKey, .isPackageKey]
        var urls: [URL] = []

        for root in roots where fm.fileExists(atPath: root.path) {
            guard let enumerator = fm.enumerator(
                at: root,
                includingPropertiesForKeys: keys,
                options: [.skipsHiddenFiles]
            ) else {
                continue
            }

            for case let url as URL in enumerator {
                if url.pathExtension == "app" {
                    urls.append(url)
                    enumerator.skipDescendants()
                }
            }
        }

        return urls
    }

    private func mountedApplicationRoots() -> [URL] {
        let volumes = URL(fileURLWithPath: "/Volumes")
        guard let entries = try? FileManager.default.contentsOfDirectory(
            at: volumes,
            includingPropertiesForKeys: [.isDirectoryKey],
            options: [.skipsHiddenFiles]
        ) else {
            return []
        }

        return entries.map { $0.appendingPathComponent("Applications") }
    }
}

@MainActor
private final class ConfigStore: ObservableObject {
    @Published var config: MappingConfig
    @Published var selectedProfile: ProfileID = .global
    @Published var status = ""

    let projectRoot: URL
    let configURL: URL
    private let legacyConfigURL: URL

    init() {
        projectRoot = findProjectRoot()
        configURL = userConfigURL()
        legacyConfigURL = projectRoot.appendingPathComponent("config/app-mapping.json")

        do {
            let url = Self.initialConfigURL(
                configURL: configURL,
                legacyConfigURL: legacyConfigURL,
                projectRoot: projectRoot
            )
            let data = try Data(contentsOf: url)
            config = try JSONDecoder().decode(MappingConfig.self, from: data)
            if url != configURL {
                save(silent: true)
                status = "Migrated config to \(configURL.path)"
            } else {
                status = "Loaded local config: \(configURL.path)"
            }
        } catch {
            config = MappingConfig(global: globalDefaultActions(), apps: [:])
            save(silent: true)
            status = "Config load failed; created a local config: \(configURL.path)"
        }
    }

    var sortedAppIDs: [String] {
        config.apps.keys.sorted()
    }

    var configuredAppIDsWithActions: [String] {
        config.apps.keys
            .filter { bundleID in
                config.apps[bundleID]?.values.contains { $0.isConfiguredAction } == true
            }
            .sorted()
    }

    func title(for profile: ProfileID) -> String {
        switch profile {
        case .global:
            return "Global default"
        case .app(let bundleID):
            return bundleID
        }
    }

    func actionBinding(_ input: KnobInput, modifierMode: ModifierMode) -> Binding<ActionConfig> {
        let key = modifierMode.mappingKey(for: input)
        return Binding(
            get: {
                switch self.selectedProfile {
                case .global:
                    return self.config.global[key] ?? ActionConfig.empty()
                case .app(let bundleID):
                    return self.config.apps[bundleID]?[key] ?? self.config.global[key] ?? ActionConfig.empty()
                }
            },
            set: { newValue in
                switch self.selectedProfile {
                case .global:
                    self.config.global[key] = newValue
                case .app(let bundleID):
                    var appActions = self.config.apps[bundleID] ?? [:]
                    appActions[key] = newValue
                    self.config.apps[bundleID] = appActions
                }
                self.save(silent: true)
            }
        )
    }

    func addApp(_ app: AppInfo) {
        addApp(bundleID: app.bundleID)
    }

    func addApp(bundleID: String) {
        let trimmed = bundleID.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmed.isEmpty else { return }

        if config.apps[trimmed] == nil {
            config.apps[trimmed] = defaultActions()
        }
        selectedProfile = .app(trimmed)
        status = "Added \(trimmed)"
        save(silent: true)
    }

    func addCurrentApp(template: ActionTemplate? = nil, modifierMode: ModifierMode = .none) {
        guard let app = NSWorkspace.shared.frontmostApplication,
              let bundleID = app.bundleIdentifier else {
            status = "No frontmost app found"
            return
        }

        addApp(bundleID: bundleID)
        if let template {
            applyTemplate(template, modifierMode: modifierMode)
        }
    }

    func applyTemplate(_ template: ActionTemplate, modifierMode: ModifierMode = .none) {
        let actions = templateActions(template)
        let modeSuffix = modifierMode == .none ? "" : " (\(modifierMode.title))"
        switch selectedProfile {
        case .global:
            if modifierMode == .none {
                config.global = actions
            } else {
                config.global = merging(actions, into: config.global, modifierMode: modifierMode)
            }
            status = "Applied template '\(template.title)' to Global default\(modeSuffix)"
        case .app(let bundleID):
            if modifierMode == .none {
                config.apps[bundleID] = actions
            } else {
                var appActions = config.apps[bundleID] ?? defaultActions()
                appActions = merging(actions, into: appActions, modifierMode: modifierMode)
                config.apps[bundleID] = appActions
            }
            status = "Applied template '\(template.title)'\(modeSuffix)"
        }
        save(silent: true)
    }

    func restoreGlobalMediaDefault() {
        config.global = globalDefaultActions()
        status = "Restored global volume default"
        save(silent: true)
    }

    func deleteSelectedApp() {
        guard case .app(let bundleID) = selectedProfile else { return }
        config.apps.removeValue(forKey: bundleID)
        selectedProfile = .global
        status = "Deleted \(bundleID)"
        save(silent: true)
    }

    func reload() {
        do {
            let data = try Data(contentsOf: configURL)
            config = try JSONDecoder().decode(MappingConfig.self, from: data)
            status = "Reloaded local config: \(configURL.path)"
        } catch {
            status = "Reload failed: \(error.localizedDescription)"
        }
    }

    func save(silent: Bool = false) {
        do {
            try FileManager.default.createDirectory(
                at: configURL.deletingLastPathComponent(),
                withIntermediateDirectories: true
            )
            let encoder = JSONEncoder()
            encoder.outputFormatting = [.prettyPrinted, .sortedKeys, .withoutEscapingSlashes]
            let data = try encoder.encode(config)
            try data.write(to: configURL)
            if !silent {
                status = "Saved local config: \(configURL.path)"
            }
        } catch {
            if !silent {
                status = "Save failed: \(error.localizedDescription)"
            }
        }
    }

    private static func initialConfigURL(configURL: URL, legacyConfigURL: URL, projectRoot: URL) -> URL {
        let exampleURL = projectRoot.appendingPathComponent("config/app-mapping.example.json")
        let fm = FileManager.default
        if fm.fileExists(atPath: configURL.path) {
            return configURL
        }
        if fm.fileExists(atPath: legacyConfigURL.path) {
            return legacyConfigURL
        }
        return exampleURL
    }

    func action(for input: KnobInput, modifierMode: ModifierMode, activeBundleID: String, activeAppName: String, lockedBundleID: String?) -> ActionResolution {
        if let lockedBundleID, lockedBundleID == activeBundleID {
            if let appActions = config.apps[activeBundleID] {
                if let action = appActions[modifierMode.mappingKey(for: input)], modifierMode != .none {
                    return ActionResolution(action: action, source: "\(activeAppName) mapping · \(modifierMode.compactTitle)")
                }
                if let action = appActions[input.rawValue] {
                    return ActionResolution(action: action, source: "\(activeAppName) mapping")
                }
            }
            if let action = config.global[modifierMode.mappingKey(for: input)], modifierMode != .none {
                return ActionResolution(action: action, source: "Global default · \(modifierMode.compactTitle)")
            }
            return ActionResolution(action: config.global[input.rawValue], source: "Global default")
        }

        if let action = config.global[modifierMode.mappingKey(for: input)], modifierMode != .none {
            return ActionResolution(action: action, source: "Global default · \(modifierMode.compactTitle)")
        }
        return ActionResolution(action: config.global[input.rawValue], source: "Global default")
    }

    private func merging(_ source: [String: ActionConfig], into target: [String: ActionConfig], modifierMode: ModifierMode) -> [String: ActionConfig] {
        var result = target
        for input in KnobInput.allCases {
            result[modifierMode.mappingKey(for: input)] = source[input.rawValue] ?? ActionConfig.empty()
        }
        return result
    }
}

@MainActor
private final class FrontmostAppModel: ObservableObject {
    @Published var name = "unknown"
    @Published var bundleID = "unknown"

    private var timer: Timer?

    func start() {
        update()
        timer = Timer.scheduledTimer(withTimeInterval: 0.5, repeats: true) { [weak self] _ in
            Task { @MainActor in self?.update() }
        }
    }

    private func update() {
        let app = NSWorkspace.shared.frontmostApplication
        name = app?.localizedName ?? "unknown"
        bundleID = app?.bundleIdentifier ?? "unknown"
    }
}

@MainActor
private final class KnobService: ObservableObject {
    @Published var isRunning = false
    @Published var deviceStatus = "Not started"
    @Published var lastEvent = "No events yet"
    @Published private(set) var lockedBundleID: String?
    @Published private(set) var lockedAppName: String?
    @Published private(set) var inputCounts: [KnobInput: Int] = [:]
    @Published private(set) var lastInput: KnobInput?
    @Published private(set) var lastModifierMode: ModifierMode = .none
    @Published private(set) var pressSequenceCount = 0
    @Published private(set) var rawEventCount = 0
    @Published private(set) var decodedEventCount = 0
    @Published private(set) var ignoredEventCount = 0
    @Published private(set) var lastRawEvent = "No HID input yet"
    @Published private(set) var recentRawEvents: [String] = []
    @Published private(set) var inputAccessStatus = "Input Monitoring: not checked"
    @Published private(set) var accessibilityStatus = "Accessibility: not checked"

    // When on, each app's mapping applies as soon as that app is frontmost and a
    // press acts immediately; the triple-press lock is not used.
    @Published var autoSwitch: Bool = UserDefaults.standard.object(forKey: "autoSwitch") as? Bool ?? true {
        didSet {
            UserDefaults.standard.set(autoSwitch, forKey: "autoSwitch")
            flushPendingPressActions()
            lastEvent = autoSwitch ? "\(clock()) Auto-switch on" : "\(clock()) Auto-switch off"
        }
    }

    private let triplePressGap: TimeInterval = 0.55
    private var manager: IOHIDManager?
    private weak var store: ConfigStore?
    private var pendingPressActions: [PendingPressAction] = []
    private var pendingPressWorkItem: DispatchWorkItem?
    private var pendingPressTargetApp: AppContext?
    private var lastExternalApp: AppContext?
    private var listenMode = "Not listening"
    private var connectedDeviceName: String?
    private var hidOpenStatus = "HID manager not opened yet"

    var lockModeTitle: String {
        if autoSwitch {
            return "Auto (frontmost app)"
        }
        if let lockedAppName {
            return "Locked to \(lockedAppName)"
        }
        return "Global default"
    }

    func effectiveMappingTitle(frontmostBundleID: String, frontmostName: String) -> String {
        if isSelfApp(bundleID: frontmostBundleID) {
            return "Global default"
        }
        if autoSwitch {
            let hasMapping = store?.configuredAppIDsWithActions.contains(frontmostBundleID) == true
            return hasMapping ? "\(frontmostName) mapping" : "Global default"
        }
        guard let lockedBundleID else {
            return "Global default"
        }
        if lockedBundleID == frontmostBundleID {
            return "\(frontmostName) mapping"
        }
        return "Global default"
    }

    func toggle(store: ConfigStore) {
        isRunning ? stop() : start(store: store)
    }

    // Builds a manager matched to the knob with all callbacks registered and
    // scheduled on the main run loop. Not opened yet.
    private func makeManager() -> IOHIDManager {
        let manager = IOHIDManagerCreate(kCFAllocatorDefault, IOOptionBits(kIOHIDOptionsTypeNone))

        IOHIDManagerSetDeviceMatching(manager, [
            kIOHIDVendorIDKey: vendorID,
            kIOHIDProductIDKey: productID
        ] as CFDictionary)

        IOHIDManagerRegisterDeviceMatchingCallback(manager, { context, _, _, device in
            guard let context else { return }
            let service = Unmanaged<KnobService>.fromOpaque(context).takeUnretainedValue()
            Task { @MainActor in service.updateConnectedDevice(deviceName(device)) }
        }, Unmanaged.passUnretained(self).toOpaque())

        IOHIDManagerRegisterDeviceRemovalCallback(manager, { context, _, _, _ in
            guard let context else { return }
            let service = Unmanaged<KnobService>.fromOpaque(context).takeUnretainedValue()
            Task { @MainActor in service.deviceStatus = "Device disconnected" }
        }, Unmanaged.passUnretained(self).toOpaque())

        IOHIDManagerRegisterInputValueCallback(manager, { context, _, _, value in
            guard let context else { return }
            let service = Unmanaged<KnobService>.fromOpaque(context).takeUnretainedValue()
            let element = IOHIDValueGetElement(value)
            let page = IOHIDElementGetUsagePage(element)
            let usage = IOHIDElementGetUsage(element)
            let intValue = IOHIDValueGetIntegerValue(value)
            let input = knobInput(page: page, usage: usage, intValue: intValue)

            Task { @MainActor in
                service.recordHIDEvent(page: page, usage: usage, intValue: intValue, input: input)
            }

            guard let input else {
                return
            }

            Task { @MainActor in service.handle(input) }
        }, Unmanaged.passUnretained(self).toOpaque())

        IOHIDManagerScheduleWithRunLoop(manager, CFRunLoopGetMain(), CFRunLoopMode.defaultMode.rawValue)
        return manager
    }

    private func discard(_ manager: IOHIDManager) {
        IOHIDManagerUnscheduleFromRunLoop(manager, CFRunLoopGetMain(), CFRunLoopMode.defaultMode.rawValue)
        IOHIDManagerClose(manager, IOOptionBits(kIOHIDOptionsTypeNone))
    }

    func start(store: ConfigStore) {
        guard !isRunning else { return }

        refreshInputAccessStatus()
        refreshAccessibilityStatus()
        guard hasInputMonitoringAccess() else {
            let granted = IOHIDRequestAccess(kIOHIDRequestTypeListenEvent)
            refreshInputAccessStatus()
            deviceStatus = "Waiting for Input Monitoring permission"
            hidOpenStatus = "Not opened: Input Monitoring permission missing"
            lastEvent = granted
                ? "\(clock()) Input Monitoring allowed; start listening again"
                : "\(clock()) Allow Input Monitoring, then start listening"
            return
        }

        self.store = store
        var manager = makeManager()
        self.manager = manager

        let seizedResult = IOHIDManagerOpen(manager, IOOptionBits(kIOHIDOptionsTypeSeizeDevice))
        if seizedResult == kIOReturnSuccess {
            listenMode = "Exclusive listening"
            deviceStatus = "Exclusive listening started"
        } else {
            // macOS only lets root seize keyboard-class devices, which is how
            // this knob enumerates over USB, the 2.4G dongle and Bluetooth.
            // Start over with a fresh manager: reopening the one whose seize
            // failed delivered no input at all.
            discard(manager)
            manager = makeManager()
            self.manager = manager
            let openResult = IOHIDManagerOpen(manager, IOOptionBits(kIOHIDOptionsTypeNone))
            guard openResult == kIOReturnSuccess else {
                discard(manager)
                deviceStatus = "Start failed: exclusive 0x\(hex(UInt32(bitPattern: seizedResult), width: 8)) / normal 0x\(hex(UInt32(bitPattern: openResult), width: 8))"
                hidOpenStatus = "Open failed: seize=0x\(hex(UInt32(bitPattern: seizedResult), width: 8)) normal=0x\(hex(UInt32(bitPattern: openResult), width: 8))"
                lastEvent = "Re-add Knob Console to Input Monitoring in System Settings"
                self.manager = nil
                return
            }
            listenMode = "Normal listening"
            if NativeKeyBlocker.start() {
                deviceStatus = "Normal listening started; the knob's own volume keys are blocked"
            } else {
                deviceStatus = "Normal listening started; system volume may still respond (allow Accessibility)"
            }
        }
        hidOpenStatus = "\(listenMode): seize=0x\(hex(UInt32(bitPattern: seizedResult), width: 8))"

        if let devices = IOHIDManagerCopyDevices(manager) as? Set<IOHIDDevice>, let first = devices.first {
            updateConnectedDevice(deviceName(first))
        }

        isRunning = true
        lastEvent = "\(clock()) Listening started"
    }

    func stop() {
        guard let manager else { return }
        flushPendingPressActions()
        NativeKeyBlocker.stop()
        discard(manager)
        self.manager = nil
        isRunning = false
        listenMode = "Not listening"
        hidOpenStatus = "Stopped"
        deviceStatus = "Stopped"
        lastEvent = "\(clock()) Listening stopped"
    }

    func clearMappingLock(showOverlay: Bool = false) {
        lockedBundleID = nil
        lockedAppName = nil
        lastEvent = "\(clock()) Global default restored"
        if showOverlay {
            showMappingStateOverlay(
                title: "Global default restored",
                subtitle: "Using global volume default",
                systemImage: "lock.open.fill"
            )
        }
    }

    func selectMappingLock(bundleID: String, appName: String) {
        lockedBundleID = bundleID
        lockedAppName = appName
        lastEvent = "\(clock()) Mapping switched: \(appName)"
        showMappingStateOverlay(
            title: "Switched to \(appName)",
            subtitle: "App mapping ready",
            systemImage: "lock.fill"
        )
    }

    func inputCount(for input: KnobInput) -> Int {
        inputCounts[input, default: 0]
    }

    func resetInputCounters() {
        inputCounts = [:]
        lastInput = nil
        lastModifierMode = .none
        pressSequenceCount = 0
        rawEventCount = 0
        decodedEventCount = 0
        ignoredEventCount = 0
        lastRawEvent = "No HID input yet"
        recentRawEvents = []
        lastEvent = "\(clock()) Input counters cleared"
    }

    func copyDebugReport(store: ConfigStore, frontmost: FrontmostAppModel) {
        refreshInputAccessStatus()
        refreshAccessibilityStatus()
        let report = debugReport(store: store, frontmost: frontmost)
        NSPasteboard.general.clearContents()
        NSPasteboard.general.setString(report, forType: .string)
        lastEvent = "\(clock()) Diagnostic report copied"
    }

    func testSystemVolume() {
        let action = ActionConfig(
            type: "key",
            description: "Volume self-test",
            keys: ["volume_up"],
            button: nil,
            clicks: nil,
            dx: nil,
            dy: nil,
            command: nil
        )
        let result = ActionExecutor.run(action)
        let detail = result.detail ?? action.description ?? "Volume self-test"
        lastEvent = "\(clock()) Volume self-test / \(detail) / \(result.ok ? "ok" : "failed")"
        OverlayPresenter.shared.show(result.overlay ?? ActionOverlayPayload(
            title: result.ok ? "Volume self-test passed" : "Volume self-test failed",
            subtitle: detail,
            systemImage: result.ok ? "speaker.wave.2.fill" : "exclamationmark.triangle.fill",
            tint: result.ok ? .accentColor : .red,
            progress: nil,
            ok: result.ok
        ))
    }

    func requestInputMonitoringAccess() {
        let granted = IOHIDRequestAccess(kIOHIDRequestTypeListenEvent)
        refreshInputAccessStatus()
        lastEvent = granted ? "\(clock()) Input Monitoring allowed" : "\(clock()) Input Monitoring requested; confirm in System Settings"
    }

    func openInputMonitoringSettings() {
        if let url = URL(string: "x-apple.systempreferences:com.apple.preference.security?Privacy_ListenEvent") {
            NSWorkspace.shared.open(url)
            lastEvent = "\(clock()) Input Monitoring settings opened"
        }
    }

    func requestAccessibilityAccess() {
        let options = [kAXTrustedCheckOptionPrompt.takeUnretainedValue() as String: true] as CFDictionary
        let granted = AXIsProcessTrustedWithOptions(options)
        refreshAccessibilityStatus()
        lastEvent = granted ? "\(clock()) Accessibility allowed" : "\(clock()) Accessibility requested; confirm in System Settings"
    }

    func openAccessibilitySettings() {
        if let url = URL(string: "x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility") {
            NSWorkspace.shared.open(url)
            lastEvent = "\(clock()) Accessibility settings opened"
        }
    }

    private func handle(_ input: KnobInput) {
        guard let store else { return }
        let app = currentAppContext()
        let modifierMode = ModifierMode.current()
        rememberExternalApp(app)
        recordInput(input, modifierMode: modifierMode)

        if input == .press, modifierMode == .none, !autoSwitch {
            handlePress(app: app, store: store)
            return
        }

        let resolution = store.action(
            for: input,
            modifierMode: modifierMode,
            activeBundleID: app.bundleID,
            activeAppName: app.name,
            lockedBundleID: effectiveLockedBundleID(for: app)
        )
        perform(input: input, modifierMode: modifierMode, app: app, resolution: resolution)
    }

    private func handlePress(app: AppContext, store: ConfigStore) {
        let now = Date()
        if let last = pendingPressActions.last,
           now.timeIntervalSince(last.date) > triplePressGap {
            flushPendingPressActions()
        }

        let targetApp = pendingPressTargetApp ?? lockTargetApp(for: app)
        pendingPressTargetApp = targetApp

        let resolution = store.action(
            for: .press,
            modifierMode: .none,
            activeBundleID: app.bundleID,
            activeAppName: app.name,
            lockedBundleID: effectiveLockedBundleID(for: app)
        )
        pendingPressActions.append(PendingPressAction(date: now, app: app, resolution: resolution))
        pressSequenceCount = pendingPressActions.count

        if pendingPressActions.count >= 3 {
            cancelPendingPressWorkItem()
            pendingPressActions.removeAll()
            pressSequenceCount = 0
            pendingPressTargetApp = nil
            toggleMappingLock(for: targetApp)
            return
        }

        lastEvent = "\(clock()) press / \(targetApp.name) / waiting for triple press (\(pendingPressActions.count)/3)"
        OverlayPresenter.shared.show(ActionOverlayPayload(
            title: "Triple press \(pendingPressActions.count)/3",
            subtitle: "Target: \(targetApp.name)",
            systemImage: "hand.tap.fill",
            tint: .accentColor,
            progress: Double(pendingPressActions.count) / 3.0,
            ok: true
        ))
        schedulePendingPressFlush()
    }

    private func toggleMappingLock(for app: AppContext) {
        guard app.bundleID != "unknown" else {
            lastEvent = "\(clock()) triple press / no frontmost app found"
            OverlayPresenter.shared.show(ActionOverlayPayload(
                title: "Triple press failed",
                subtitle: "No frontmost app found",
                systemImage: "exclamationmark.triangle.fill",
                tint: .orange,
                progress: nil,
                ok: false
            ))
            return
        }
        guard !isSelfApp(bundleID: app.bundleID) else {
            lockedBundleID = nil
            lockedAppName = nil
            lastEvent = "\(clock()) triple press / console uses Global default"
            showMappingStateOverlay(
                title: "Global default restored",
                subtitle: "App lock is not used for the console",
                systemImage: "lock.open.fill"
            )
            return
        }

        if lockedBundleID == app.bundleID {
            lockedBundleID = nil
            lockedAppName = nil
            lastEvent = "\(clock()) triple press / Global default restored"
            showMappingStateOverlay(
                title: "Global default restored",
                subtitle: app.name,
                systemImage: "lock.open.fill"
            )
        } else {
            lockedBundleID = app.bundleID
            lockedAppName = app.name
            lastEvent = "\(clock()) triple press / locked to \(app.name)"
            showMappingStateOverlay(
                title: "Locked to \(app.name)",
                subtitle: "App mapping active",
                systemImage: "lock.fill"
            )
        }
    }

    private func showMappingStateOverlay(title: String, subtitle: String, systemImage: String) {
        OverlayPresenter.shared.show(ActionOverlayPayload(
            title: title,
            subtitle: subtitle,
            systemImage: systemImage,
            tint: .accentColor,
            progress: nil,
            ok: true
        ))
    }

    private func schedulePendingPressFlush() {
        cancelPendingPressWorkItem()
        let workItem = DispatchWorkItem { [weak self] in
            Task { @MainActor in
                self?.flushPendingPressActions()
            }
        }
        pendingPressWorkItem = workItem
        DispatchQueue.main.asyncAfter(deadline: .now() + triplePressGap, execute: workItem)
    }

    private func cancelPendingPressWorkItem() {
        pendingPressWorkItem?.cancel()
        pendingPressWorkItem = nil
    }

    private func flushPendingPressActions() {
        cancelPendingPressWorkItem()
        let actions = pendingPressActions
        pendingPressActions.removeAll()
        pendingPressTargetApp = nil
        pressSequenceCount = 0

        for pending in actions {
            perform(input: .press, modifierMode: .none, app: pending.app, resolution: pending.resolution)
        }
    }

    private func rememberExternalApp(_ app: AppContext) {
        guard app.bundleID != "unknown", !isSelfApp(bundleID: app.bundleID) else { return }
        lastExternalApp = app
    }

    private func lockTargetApp(for app: AppContext) -> AppContext {
        if app.bundleID != "unknown", !isSelfApp(bundleID: app.bundleID) {
            return app
        }
        return lastExternalApp ?? app
    }

    private func recordInput(_ input: KnobInput, modifierMode: ModifierMode) {
        inputCounts[input, default: 0] += 1
        lastInput = input
        lastModifierMode = modifierMode
    }

    private func recordHIDEvent(page: UInt32, usage: UInt32, intValue: Int, input: KnobInput?) {
        rawEventCount += 1
        if input != nil {
            decodedEventCount += 1
        } else {
            ignoredEventCount += 1
        }

        let decoded = input.map { " -> \($0.title)" } ?? ""
        lastRawEvent = "page=0x\(hex(page, width: 4)) usage=0x\(hex(usage, width: 4)) int=\(intValue)\(decoded)"
        recentRawEvents.append("\(clock()) \(lastRawEvent)")
        if recentRawEvents.count > 80 {
            recentRawEvents.removeFirst(recentRawEvents.count - 80)
        }
    }

    private func refreshInputAccessStatus() {
        let access = IOHIDCheckAccess(kIOHIDRequestTypeListenEvent)
        if access == kIOHIDAccessTypeGranted {
            inputAccessStatus = "Input Monitoring: allowed"
        } else if access == kIOHIDAccessTypeDenied {
            inputAccessStatus = "Input Monitoring: not allowed"
        } else {
            inputAccessStatus = "Input Monitoring: undetermined"
        }
    }

    private func hasInputMonitoringAccess() -> Bool {
        IOHIDCheckAccess(kIOHIDRequestTypeListenEvent) == kIOHIDAccessTypeGranted
    }

    private func debugReport(store: ConfigStore, frontmost: FrontmostAppModel) -> String {
        var lines: [String] = []
        let bundle = Bundle.main
        let officialApps = NSWorkspace.shared.runningApplications.filter {
            ($0.bundleIdentifier ?? "").lowercased() == "com.lqkj.keyboard.anticater"
                || ($0.localizedName ?? "").lowercased().contains("anticater")
        }

        lines.append("ANTICATER Knob Console diagnostic report")
        lines.append("Generated: \(Date())")
        lines.append("")
        lines.append("App")
        lines.append("- Name: \(bundle.object(forInfoDictionaryKey: "CFBundleName") as? String ?? "unknown")")
        lines.append("- Bundle ID: \(bundle.bundleIdentifier ?? "unknown")")
        lines.append("- Version: \(bundle.object(forInfoDictionaryKey: "CFBundleShortVersionString") as? String ?? "unknown") (\(bundle.object(forInfoDictionaryKey: "CFBundleVersion") as? String ?? "unknown"))")
        lines.append("- Path: \(bundle.bundlePath)")
        lines.append("")
        lines.append("Permissions / status")
        lines.append("- Input Monitoring: \(inputAccessStatus)")
        lines.append("- Accessibility: \(accessibilityStatus)")
        lines.append("- Listening: \(isRunning ? "yes" : "no")")
        lines.append("- Device status: \(deviceStatus)")
        lines.append("- HID open status: \(hidOpenStatus)")
        lines.append("- Connected device: \(connectedDeviceName ?? "none")")
        lines.append("- Last action: \(lastEvent)")
        lines.append("- Frontmost app: \(frontmost.name) / \(frontmost.bundleID)")
        lines.append("- Mapping mode: \(lockModeTitle)")
        lines.append("- Config file: \(store.configURL.path)")
        lines.append("")
        lines.append("Official ANTICATER")
        if officialApps.isEmpty {
            lines.append("- Official ANTICATER app is not running")
        } else {
            for app in officialApps {
                lines.append("- Running: pid=\(app.processIdentifier) name=\(app.localizedName ?? "unknown") bundle=\(app.bundleIdentifier ?? "unknown") path=\(app.bundleURL?.path ?? "unknown")")
            }
        }
        lines.append("")
        lines.append("Target device")
        lines.append("- Target VendorID: 0x\(hex(vendorID, width: 4))")
        lines.append("- Target ProductID: 0x\(hex(productID, width: 4))")
        lines.append("- System HID enumeration:")
        let devices = diagnosticHIDDevices()
        if devices.isEmpty {
            lines.append("  - No target or likely ANTICATER/CXKJ device enumerated")
        } else {
            lines.append(contentsOf: devices.map { "  - \($0)" })
        }
        lines.append("")
        lines.append("Input statistics")
        lines.append("- Raw events: \(rawEventCount)")
        lines.append("- Recognized: \(decodedEventCount)")
        lines.append("- Ignored: \(ignoredEventCount)")
        lines.append("- Last input: \(lastInput.map { lastModifierMode.gestureTitle(for: $0) } ?? "none")")
        lines.append("- Last HID: \(lastRawEvent)")
        for input in KnobInput.allCases {
            lines.append("- \(input.title): \(inputCounts[input, default: 0])")
        }
        lines.append("")
        lines.append("Recent raw HID events")
        if recentRawEvents.isEmpty {
            lines.append("- None")
        } else {
            lines.append(contentsOf: recentRawEvents.suffix(40).map { "- \($0)" })
        }
        lines.append("")
        lines.append("Troubleshooting")
        lines.append("- If the system HID enumeration has no 0x\(hex(vendorID, width: 4))/0x\(hex(productID, width: 4)), the paired device or its firmware/driver may use a different VID/PID.")
        lines.append("- If Input Monitoring is not allowed, the app sees the device but receives no input.")
        lines.append("- If raw events are counted but recognized is 0, send the recent raw HID events so the mapping can be added.")
        lines.append("- If the official ANTICATER app is running, quit it and try again.")
        return lines.joined(separator: "\n")
    }

    private func perform(input: KnobInput, modifierMode: ModifierMode, app: AppContext, resolution: ActionResolution) {
        let gestureTitle = modifierMode.gestureTitle(for: input)
        guard let action = resolution.action else {
            lastEvent = "\(clock()) \(gestureTitle) / \(app.name) / \(resolution.source) / no action"
            return
        }

        refreshAccessibilityStatus()
        if ActionExecutor.needsAccessibility(action), !AXIsProcessTrusted() {
            lastEvent = "\(clock()) \(gestureTitle) / \(app.name) / \(resolution.source) / Accessibility permission needed / failed"
            return
        }

        let result = ActionExecutor.run(action)
        let detail = result.detail ?? (action.description?.isEmpty == false ? action.description! : action.type)
        lastEvent = "\(clock()) \(gestureTitle) / \(app.name) / \(resolution.source) / \(detail) / \(result.ok ? "ok" : "failed")"
        let overlay = result.overlay ?? ActionOverlayPayload(
            title: result.ok ? detail : "Action failed",
            subtitle: "\(gestureTitle) · \(resolution.source)",
            systemImage: result.ok ? overlaySystemImage(for: input) : "exclamationmark.triangle.fill",
            tint: result.ok ? .accentColor : .red,
            progress: nil,
            ok: result.ok
        )
        if overlay.style == .macVolume {
            OverlayPresenter.shared.show(overlay)
        }
    }

    private func currentAppContext() -> AppContext {
        let app = NSWorkspace.shared.frontmostApplication
        return AppContext(
            name: app?.localizedName ?? "unknown",
            bundleID: app?.bundleIdentifier ?? "unknown"
        )
    }

    private func effectiveLockedBundleID(for app: AppContext) -> String? {
        if isSelfApp(bundleID: app.bundleID) {
            return nil
        }
        return autoSwitch ? app.bundleID : lockedBundleID
    }

    private func isSelfApp(bundleID: String) -> Bool {
        bundleID == Bundle.main.bundleIdentifier
    }

    private func updateConnectedDevice(_ name: String) {
        connectedDeviceName = name
        deviceStatus = "\(listenMode): \(name)"
    }

    private func refreshAccessibilityStatus() {
        accessibilityStatus = AXIsProcessTrusted() ? "Accessibility: allowed" : "Accessibility: not allowed"
    }
}

// Without a seize, macOS still acts on the knob's volume / mute / brightness
// keys. This drops those system-defined key events when one of the knob's own
// HID reports carries about the same timestamp, so only the mapped action
// runs. Keys from a real keyboard are held briefly and re-posted when no knob
// report claims them.
//
// The tap and a second, listen-only HID manager live on their own thread and
// compare the events' own timestamps: the main thread can lag by hundreds of
// milliseconds while actions run, which made arrival times useless.
private enum NativeKeyBlocker {
    static let ownEventTag: Int64 = 0x4B4E4F42 // "KNOB": events this app posts
    private static let matchWindow: UInt64 = 60_000_000 // ns between report and key
    private static let holdTime: TimeInterval = 0.05
    // NX_KEYTYPE_ sound up, sound down, brightness up, brightness down, mute, play
    private static let blockedKeyTypes: Set<Int> = [0, 1, 2, 3, 7, 16]

    private static var thread: Thread?
    private static var runLoop: CFRunLoop?
    private static var tap: CFMachPort?
    // The rest is only touched on the blocker thread.
    private static var reportTimes: [UInt64] = []
    private static var held: [(event: CGEvent, time: UInt64, heldAt: UInt64)] = []
    private static let timebase: mach_timebase_info_data_t = {
        var info = mach_timebase_info_data_t()
        mach_timebase_info(&info)
        return info
    }()

    static func start() -> Bool {
        guard thread == nil else { return true }
        // NX_SYSDEFINED for the media keys; key down / up for the knob's F13.
        let mask = (CGEventMask(1) << 14) | (CGEventMask(1) << CGEventType.keyDown.rawValue) | (CGEventMask(1) << CGEventType.keyUp.rawValue)
        guard let port = CGEvent.tapCreate(
            tap: .cgSessionEventTap,
            place: .headInsertEventTap,
            options: .defaultTap,
            eventsOfInterest: mask,
            callback: { _, type, event, _ in NativeKeyBlocker.handle(type: type, event: event) },
            userInfo: nil
        ) else { return false }

        let ready = DispatchSemaphore(value: 0)
        let worker = Thread {
            guard let loop = CFRunLoopGetCurrent() else {
                ready.signal()
                return
            }
            runLoop = loop
            reportTimes.removeAll()
            held.removeAll()
            let source = CFMachPortCreateRunLoopSource(kCFAllocatorDefault, port, 0)
            CFRunLoopAddSource(loop, source, .commonModes)

            let manager = IOHIDManagerCreate(kCFAllocatorDefault, IOOptionBits(kIOHIDOptionsTypeNone))
            IOHIDManagerSetDeviceMatching(manager, [
                kIOHIDVendorIDKey: vendorID,
                kIOHIDProductIDKey: productID
            ] as CFDictionary)
            IOHIDManagerRegisterInputValueCallback(manager, { _, _, _, value in
                NativeKeyBlocker.noteReport(machTime: IOHIDValueGetTimeStamp(value))
            }, nil)
            IOHIDManagerScheduleWithRunLoop(manager, loop, CFRunLoopMode.defaultMode.rawValue)
            IOHIDManagerOpen(manager, IOOptionBits(kIOHIDOptionsTypeNone))

            CGEvent.tapEnable(tap: port, enable: true)
            ready.signal()
            CFRunLoopRun()

            CGEvent.tapEnable(tap: port, enable: false)
            IOHIDManagerUnscheduleFromRunLoop(manager, loop, CFRunLoopMode.defaultMode.rawValue)
            IOHIDManagerClose(manager, IOOptionBits(kIOHIDOptionsTypeNone))
            CFRunLoopRemoveSource(loop, source, .commonModes)
            CFMachPortInvalidate(port)
        }
        worker.name = "NativeKeyBlocker"
        worker.qualityOfService = .userInteractive
        tap = port
        thread = worker
        worker.start()
        ready.wait()
        return true
    }

    static func stop() {
        guard thread != nil else { return }
        if let runLoop {
            CFRunLoopStop(runLoop)
        }
        thread = nil
        runLoop = nil
        tap = nil
    }

    private static func nanoseconds(machTime: UInt64) -> UInt64 {
        machTime * UInt64(timebase.numer) / UInt64(timebase.denom)
    }

    private static func noteReport(machTime: UInt64) {
        reportTimes.append(nanoseconds(machTime: machTime))
        if reportTimes.count > 64 {
            reportTimes.removeFirst(reportTimes.count - 64)
        }
    }

    private static func knobReported(near time: UInt64) -> Bool {
        reportTimes.contains { $0 > time ? $0 - time < matchWindow : time - $0 < matchWindow }
    }

    private static func handle(type: CGEventType, event: CGEvent) -> Unmanaged<CGEvent>? {
        if type == .tapDisabledByTimeout || type == .tapDisabledByUserInput {
            if let tap { CGEvent.tapEnable(tap: tap, enable: true) }
            return Unmanaged.passUnretained(event)
        }
        if type == .keyDown || type == .keyUp {
            // F13 is a key no Mac keyboard has; the knob sends it for hold + turn
            // (see knobInput). Drop it so it reaches no other app.
            let isF13 = event.getIntegerValueField(.keyboardEventKeycode) == 0x69 // kVK_F13
            return isF13 && event.getIntegerValueField(.eventSourceUserData) != ownEventTag ? nil : Unmanaged.passUnretained(event)
        }
        guard type.rawValue == 14,
              event.getIntegerValueField(.eventSourceUserData) != ownEventTag,
              let nsEvent = NSEvent(cgEvent: event),
              nsEvent.subtype.rawValue == 8,
              blockedKeyTypes.contains((nsEvent.data1 & 0xFFFF0000) >> 16)
        else {
            return Unmanaged.passUnretained(event)
        }
        let time = event.timestamp
        if knobReported(near: time) {
            return nil // the knob's own key
        }
        // Its report may not have been delivered yet: hold the key, then decide.
        guard let copy = event.copy(), let runLoop else {
            return Unmanaged.passUnretained(event)
        }
        held.append((copy, time, DispatchTime.now().uptimeNanoseconds))
        let timer = CFRunLoopTimerCreateWithHandler(kCFAllocatorDefault, CFAbsoluteTimeGetCurrent() + holdTime, 0, 0, 0) { _ in
            releaseHeld()
        }
        CFRunLoopAddTimer(runLoop, timer, .commonModes)
        return nil
    }

    private static func releaseHeld() {
        let now = DispatchTime.now().uptimeNanoseconds
        let minimumHold = UInt64(holdTime * 1_000_000_000) - 5_000_000
        var keep: [(event: CGEvent, time: UInt64, heldAt: UInt64)] = []
        for item in held {
            if now - item.heldAt < minimumHold {
                keep.append(item)
            } else if !knobReported(near: item.time) {
                // No knob report around it: a real keyboard key, pass it on.
                item.event.setIntegerValueField(.eventSourceUserData, value: ownEventTag)
                item.event.post(tap: .cgSessionEventTap)
            }
        }
        held = keep
    }
}

private let mainWindowID = "main"

// Reopens the main window when it has been closed; activating the app alone
// does not bring a closed window back.
private struct ShowConsoleButton: View {
    @Environment(\.openWindow) private var openWindow

    var body: some View {
        Button("Show console") {
            openWindow(id: mainWindowID)
            NSApp.activate(ignoringOtherApps: true)
        }
    }
}

@main
private struct KnobControlApp: App {
    @StateObject private var store = ConfigStore()
    @StateObject private var frontmost = FrontmostAppModel()
    @StateObject private var knobService = KnobService()
    @StateObject private var appCatalog = AppCatalog()

    var body: some Scene {
        Window("Knob Console", id: mainWindowID) {
            ContentView()
                .environmentObject(store)
                .environmentObject(frontmost)
                .environmentObject(knobService)
                .environmentObject(appCatalog)
                .frame(minWidth: 980, minHeight: 640)
                .onAppear {
                    frontmost.start()
                    appCatalog.refresh()
                }
        }
        .windowStyle(.titleBar)

        MenuBarExtra("Knob Console", systemImage: "slider.horizontal.3") {
            ShowConsoleButton()
            Button(knobService.isRunning ? "Stop listening" : "Start listening") {
                knobService.toggle(store: store)
            }

            Toggle("Auto-switch by frontmost app", isOn: $knobService.autoSwitch)

            Menu("Switch mapping") {
                Button {
                    knobService.clearMappingLock(showOverlay: true)
                } label: {
                    Label("Global default", systemImage: knobService.lockedBundleID == nil ? "checkmark.circle.fill" : "circle")
                }

                if store.configuredAppIDsWithActions.isEmpty {
                    Text("No configured apps")
                } else {
                    Divider()
                    ForEach(store.configuredAppIDsWithActions, id: \.self) { bundleID in
                        let info = appCatalog.app(bundleID: bundleID)
                        Button {
                            knobService.selectMappingLock(
                                bundleID: bundleID,
                                appName: info?.name ?? bundleID
                            )
                        } label: {
                            Label(
                                info?.name ?? bundleID,
                                systemImage: knobService.lockedBundleID == bundleID ? "checkmark.circle.fill" : "app"
                            )
                        }
                    }
                }
            }

            Divider()
            Text(knobService.deviceStatus)
            Text(knobService.inputAccessStatus)
            Text(knobService.accessibilityStatus)
            Text("Mapping mode: \(knobService.lockModeTitle)")
            Text(knobService.lastEvent)
            Divider()
            Button("Quit") {
                knobService.stop()
                NSApp.terminate(nil)
            }
        }
    }
}

private struct ContentView: View {
    @EnvironmentObject private var store: ConfigStore
    @EnvironmentObject private var frontmost: FrontmostAppModel
    @EnvironmentObject private var knobService: KnobService
    @EnvironmentObject private var appCatalog: AppCatalog
    @State private var newBundleID = ""
    @State private var selectedTemplate: ActionTemplate = .browser
    @State private var selectedModifierMode: ModifierMode = .none

    var body: some View {
        NavigationSplitView {
            sidebar
                .navigationSplitViewColumnWidth(min: 230, ideal: 280)
        } detail: {
            VStack(spacing: 0) {
                header
                Divider()
                editor
                Divider()
                footer
            }
        }
    }

    private var sidebar: some View {
        VStack(spacing: 12) {
            List(selection: $store.selectedProfile) {
                Section("Default") {
                    Text("Global default").tag(ProfileID.global)
                }

                Section("Apps") {
                    ForEach(store.sortedAppIDs, id: \.self) { bundleID in
                        ConfiguredAppRow(info: appCatalog.app(bundleID: bundleID), fallbackBundleID: bundleID)
                        .tag(ProfileID.app(bundleID))
                    }
                }

                Section("Discovered apps") {
                    ForEach(appCatalog.filteredApps) { app in
                        Button {
                            store.addApp(app)
                        } label: {
                            DiscoveredAppRow(app: app, configured: store.config.apps[app.bundleID] != nil)
                        }
                        .buttonStyle(.plain)
                    }
                }
            }

            VStack(spacing: 8) {
                TextField("Search apps", text: $appCatalog.query)
                    .textFieldStyle(.roundedBorder)
                HStack {
                    Button("Refresh") {
                        appCatalog.refresh()
                    }
                    Text(appCatalog.status)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                        .lineLimit(1)
                }
                TextField("Enter a bundle ID manually", text: $newBundleID)
                    .textFieldStyle(.roundedBorder)
                HStack {
                    Button("Add") {
                        store.addApp(bundleID: newBundleID)
                        newBundleID = ""
                    }
                    Button("Add current app") {
                        store.addCurrentApp()
                    }
                }
                Button("Delete selected app", role: .destructive) {
                    store.deleteSelectedApp()
                }
                .disabled(!isAppSelected)
            }
            .padding([.horizontal, .bottom], 12)
        }
    }

    private var header: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack(alignment: .center) {
                VStack(alignment: .leading, spacing: 4) {
                    Text(selectedProfileTitle)
                        .font(.title2.weight(.semibold))
                    Text("Frontmost app: \(frontmost.name)")
                        .font(.callout)
                        .foregroundStyle(.secondary)
                }

                Spacer()

                Button(knobService.isRunning ? "Stop listening" : "Start listening") {
                    knobService.toggle(store: store)
                }

                Button("Reload") {
                    store.reload()
                }

                Button("Save config") {
                    store.save()
                }
            }

            StatusDashboard(
                running: knobService.isRunning,
                deviceStatus: knobService.deviceStatus,
                frontmostName: frontmost.name,
                lockMode: knobService.lockModeTitle,
                effectiveMapping: knobService.effectiveMappingTitle(
                    frontmostBundleID: frontmost.bundleID,
                    frontmostName: frontmost.name
                ),
                inputAccessStatus: knobService.inputAccessStatus,
                accessibilityStatus: knobService.accessibilityStatus,
                inputCounts: knobService.inputCounts,
                lastInput: knobService.lastInput,
                lastModifierMode: knobService.lastModifierMode,
                pressSequenceCount: knobService.pressSequenceCount,
                rawEventCount: knobService.rawEventCount,
                decodedEventCount: knobService.decodedEventCount,
                ignoredEventCount: knobService.ignoredEventCount,
                lastRawEvent: knobService.lastRawEvent,
                lastEvent: knobService.lastEvent
            )

            templateBar
        }
        .padding(18)
    }

    private var templateBar: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack(spacing: 10) {
                Text("Edit mode")
                    .font(.caption.weight(.semibold))
                    .foregroundStyle(.secondary)
                    .fixedSize()

                Picker("Modifier", selection: $selectedModifierMode) {
                    ForEach(ModifierMode.allCases) { mode in
                        Text(mode.title).tag(mode)
                    }
                }
                .frame(width: 230)

                Text(selectedModifierMode == .none ? "Editing plain knob gestures" : "Editing \(selectedModifierMode.title) + knob gestures")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .lineLimit(1)

                Spacer(minLength: 0)

                toolMenus
            }

            // One row when it fits, otherwise the buttons wrap onto a second row.
            ViewThatFits(in: .horizontal) {
                HStack(spacing: 10) {
                    templatePicker
                    templateButtons
                    Spacer(minLength: 0)
                }
                VStack(alignment: .leading, spacing: 8) {
                    HStack(spacing: 10) {
                        templatePicker
                        Spacer(minLength: 0)
                    }
                    HStack(spacing: 10) {
                        templateButtons
                        Spacer(minLength: 0)
                    }
                }
            }
        }
    }

    @ViewBuilder
    private var templatePicker: some View {
        Text("Quick templates")
            .font(.caption.weight(.semibold))
            .foregroundStyle(.secondary)
            .fixedSize()

        Picker("Template", selection: $selectedTemplate) {
            ForEach(ActionTemplate.allCases) { template in
                Text(template.title).tag(template)
            }
        }
        .pickerStyle(.segmented)
        .labelsHidden()
        .fixedSize()
        .help(selectedTemplate.subtitle)
    }

    @ViewBuilder
    private var templateButtons: some View {
        Button("Apply to current profile") {
            store.applyTemplate(selectedTemplate, modifierMode: selectedModifierMode)
        }
        .fixedSize()

        Button("Create for frontmost app and apply") {
            store.addCurrentApp(template: selectedTemplate, modifierMode: selectedModifierMode)
        }
        .fixedSize()
    }

    // Sized to their titles; a Menu otherwise stretches across the whole row.
    @ViewBuilder
    private var toolMenus: some View {
        Menu("Listening tools") {
            Button("Restore global volume default") {
                store.restoreGlobalMediaDefault()
            }

            Divider()

            Toggle("Auto-switch by frontmost app", isOn: $knobService.autoSwitch)

            Button("Release triple-press lock") {
                knobService.clearMappingLock(showOverlay: true)
            }
            .disabled(knobService.autoSwitch || knobService.lockedBundleID == nil)

            Button("Clear counters") {
                knobService.resetInputCounters()
            }

            Button("Volume self-test") {
                knobService.testSystemVolume()
            }

            Divider()

            Button("Copy diagnostic report") {
                knobService.copyDebugReport(store: store, frontmost: frontmost)
            }
        }
        .fixedSize()

        Menu("Permissions") {
            Button("Request permission") {
                knobService.requestInputMonitoringAccess()
            }

            Button("Open Input Monitoring") {
                knobService.openInputMonitoringSettings()
            }

            Button("Request Accessibility") {
                knobService.requestAccessibilityAccess()
            }

            Button("Open Accessibility") {
                knobService.openAccessibilitySettings()
            }
        }
        .fixedSize()
    }

    private var editor: some View {
        ScrollView {
            VStack(spacing: 14) {
                ForEach(KnobInput.allCases) { input in
                    ActionEditorRow(
                        input: input,
                        modifierMode: selectedModifierMode,
                        action: store.actionBinding(input, modifierMode: selectedModifierMode)
                    )
                }
            }
            .padding(18)
        }
    }

    private var footer: some View {
        HStack {
            Text(store.status)
                .font(.caption)
                .foregroundStyle(.secondary)
            Spacer()
            Text("Config: \(store.configURL.lastPathComponent)")
                .font(.caption2)
                .foregroundStyle(.tertiary)
                .help(store.configURL.path)
        }
        .padding(.horizontal, 18)
        .padding(.vertical, 10)
    }

    private var isAppSelected: Bool {
        if case .app = store.selectedProfile { return true }
        return false
    }

    private var selectedProfileTitle: String {
        switch store.selectedProfile {
        case .global:
            return "Global default"
        case .app(let bundleID):
            return appCatalog.app(bundleID: bundleID)?.name ?? store.title(for: store.selectedProfile)
        }
    }
}

private struct ConfiguredAppRow: View {
    let info: AppInfo?
    let fallbackBundleID: String

    var body: some View {
        HStack(spacing: 8) {
            AppIconView(path: info?.path, size: 24)
            VStack(alignment: .leading, spacing: 2) {
                Text(info?.name ?? fallbackBundleID)
                    .font(.system(size: 12, weight: .medium))
                    .lineLimit(1)
                Text(info?.isRunning == true ? "Custom mapping · running" : "Custom mapping")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
        }
    }
}

private struct DiscoveredAppRow: View {
    let app: AppInfo
    let configured: Bool

    var body: some View {
        HStack(spacing: 8) {
            AppIconView(path: app.path, size: 22)
            VStack(alignment: .leading, spacing: 2) {
                HStack(spacing: 6) {
                    Text(app.name)
                        .font(.system(size: 12, weight: .medium))
                        .lineLimit(1)
                    if app.isRunning {
                        Text("Running")
                            .font(.caption2)
                            .padding(.horizontal, 5)
                            .padding(.vertical, 1)
                            .background(.green.opacity(0.18), in: Capsule())
                    }
                    if configured {
                        Text("Added")
                            .font(.caption2)
                            .padding(.horizontal, 5)
                            .padding(.vertical, 1)
                            .background(.blue.opacity(0.16), in: Capsule())
                    }
                }
                Text(configured ? "Already in the app list on the left" : "Click to add to the app list")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .lineLimit(1)
            }
        }
    }
}

private struct AppIconView: View {
    let path: String?
    let size: CGFloat

    var body: some View {
        Image(nsImage: icon)
            .resizable()
            .frame(width: size, height: size)
            .cornerRadius(5)
    }

    private var icon: NSImage {
        if let path, !path.isEmpty {
            return NSWorkspace.shared.icon(forFile: path)
        }
        return NSWorkspace.shared.icon(forFileType: "app")
    }
}

// Ready-made actions offered by the "Quick pick" menu on every gesture row.
private struct QuickPick {
    let title: String
    let action: ActionConfig

    static func keys(_ title: String, _ keys: String...) -> QuickPick {
        QuickPick(title: title, action: ActionConfig(
            type: keys.count == 1 ? "key" : "shortcut", description: title, keys: keys,
            button: nil, clicks: nil, dx: nil, dy: nil, command: nil
        ))
    }

    static func scroll(_ title: String, dx: Int = 0, dy: Int = 0) -> QuickPick {
        QuickPick(title: title, action: ActionConfig(
            type: "scroll", description: title, keys: nil,
            button: nil, clicks: nil, dx: dx, dy: dy, command: nil
        ))
    }
}

private let quickPickGroups: [(title: String, items: [QuickPick])] = [
    ("Media", [
        .keys("Volume Up", "volume_up"),
        .keys("Volume Down", "volume_down"),
        .keys("Mute", "mute"),
        .keys("Play / Pause", "play_pause"),
        .keys("Next Track", "next_track"),
        .keys("Previous Track", "previous_track"),
        .keys("Brightness Up", "brightness_up"),
        .keys("Brightness Down", "brightness_down")
    ]),
    ("View", [
        .keys("Zoom In (⌘=)", "command", "="),
        .keys("Zoom Out (⌘−)", "command", "-"),
        .scroll("Scroll Up", dy: 3),
        .scroll("Scroll Down", dy: -3),
        .scroll("Scroll Left", dx: -3),
        .scroll("Scroll Right", dx: 3)
    ]),
    ("Navigation", [
        .keys("Next Tab (⌃⇥)", "control", "tab"),
        .keys("Previous Tab (⌃⇧⇥)", "control", "shift", "tab"),
        .keys("Back (⌘[)", "command", "["),
        .keys("Forward (⌘])", "command", "]"),
        .keys("Reload (⌘R)", "command", "r")
    ]),
    ("Edit", [
        .keys("Undo (⌘Z)", "command", "z"),
        .keys("Redo (⇧⌘Z)", "command", "shift", "z"),
        .keys("Copy (⌘C)", "command", "c"),
        .keys("Paste (⌘V)", "command", "v"),
        .keys("Save (⌘S)", "command", "s"),
        .keys("Find (⌘F)", "command", "f")
    ]),
    ("Keys", [
        .keys("Left Arrow", "left"),
        .keys("Right Arrow", "right"),
        .keys("Up Arrow", "up"),
        .keys("Down Arrow", "down"),
        .keys("Return", "return"),
        .keys("Esc", "escape"),
        .keys("Space", "space")
    ]),
    ("Other", [
        QuickPick(title: "Left Click", action: ActionConfig(
            type: "mouse", description: "Left Click", keys: nil,
            button: "left", clicks: 1, dx: nil, dy: nil, command: nil
        )),
        QuickPick(title: "No Action", action: .empty())
    ])
]

private struct ActionEditorRow: View {
    let input: KnobInput
    let modifierMode: ModifierMode
    @Binding var action: ActionConfig
    @State private var showShortcutRecorder = false

    private var keysText: Binding<String> {
        Binding(
            get: { displayKeyList(action.keys ?? []) },
            set: { text in
                action.keys = parseKeyList(text)
            }
        )
    }

    private var actionType: Binding<String> {
        Binding(
            get: { action.type },
            set: { action.type = $0 }
        )
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack {
                VStack(alignment: .leading, spacing: 4) {
                    HStack(spacing: 8) {
                        Text(gestureSymbol(input))
                            .font(.title3.weight(.semibold))
                            .frame(width: 24)
                        Text(modifierMode == .none ? input.title : "\(modifierMode.compactTitle) + \(input.title)")
                            .font(.headline)
                    }
                    Text(actionSummary(action))
                        .font(.caption)
                        .foregroundStyle(.secondary)
                        .lineLimit(1)
                }
                .frame(width: 150, alignment: .leading)

                Picker("Type", selection: actionType) {
                    ForEach(ActionType.allCases) { type in
                        Text(type.title).tag(type.rawValue)
                    }
                }
                .pickerStyle(.segmented)

                TextField("Description", text: optionalString($action.description))
                    .textFieldStyle(.roundedBorder)
                    .frame(minWidth: 140)

                Menu("Quick pick") {
                    ForEach(Array(quickPickGroups.enumerated()), id: \.offset) { _, group in
                        Section(group.title) {
                            ForEach(Array(group.items.enumerated()), id: \.offset) { _, item in
                                Button(item.title) {
                                    action = item.action
                                }
                            }
                        }
                    }
                }
                .frame(width: 110)
            }

            fields
        }
        .padding(14)
        .background(.quaternary.opacity(0.45), in: RoundedRectangle(cornerRadius: 8))
        .sheet(isPresented: $showShortcutRecorder) {
            ShortcutRecorderSheet(input: input, modifierMode: modifierMode) { keys in
                action.keys = keys
                showShortcutRecorder = false
            } onCancel: {
                showShortcutRecorder = false
            }
        }
    }

    @ViewBuilder
    private var fields: some View {
        switch action.type {
        case "shortcut", "key":
            HStack {
                Text("Key")
                    .frame(width: 96, alignment: .leading)
                    .foregroundStyle(.secondary)
                TextField("Type manually, comma-separated", text: keysText)
                    .textFieldStyle(.roundedBorder)
                Button("Record shortcut") {
                    showShortcutRecorder = true
                }
                Button("Clear") {
                    action.keys = []
                }
            }
        case "mouse":
            HStack {
                Text("Mouse")
                    .frame(width: 96, alignment: .leading)
                    .foregroundStyle(.secondary)
                Picker("Button", selection: optionalString($action.button)) {
                    Text("Left").tag("left")
                    Text("Right").tag("right")
                    Text("Middle").tag("middle")
                }
                .frame(width: 160)
                Stepper("Clicks: \(action.clicks ?? 1)", value: optionalInt($action.clicks, defaultValue: 1), in: 1...5)
            }
        case "scroll":
            HStack {
                Text("Scroll")
                    .frame(width: 96, alignment: .leading)
                    .foregroundStyle(.secondary)
                Stepper("Horizontal \(action.dx ?? 0)", value: optionalInt($action.dx, defaultValue: 0), in: -20...20)
                Stepper("Vertical \(action.dy ?? 0)", value: optionalInt($action.dy, defaultValue: 0), in: -20...20)
            }
        case "shell":
            HStack {
                Text("Shell")
                    .frame(width: 96, alignment: .leading)
                    .foregroundStyle(.secondary)
                TextField("Enter the command to run", text: optionalString($action.command))
                    .textFieldStyle(.roundedBorder)
            }
        default:
            HStack {
                Text("No action")
                    .frame(width: 96, alignment: .leading)
                    .foregroundStyle(.secondary)
                Text("This gesture is ignored.")
                    .foregroundStyle(.secondary)
            }
        }
    }

}

private struct ShortcutRecorderSheet: View {
    let input: KnobInput
    let modifierMode: ModifierMode
    let onCapture: ([String]) -> Void
    let onCancel: () -> Void
    @State private var preview = "Waiting for input"
    @State private var monitor: Any?

    var body: some View {
        VStack(spacing: 18) {
            Text("Record shortcut")
                .font(.title2.weight(.semibold))
            Text("Press the shortcut to bind to '\(modifierMode.gestureTitle(for: input))'")
                .foregroundStyle(.secondary)
            Text(preview)
                .font(.system(size: 28, weight: .semibold, design: .rounded))
                .frame(maxWidth: .infinity)
                .padding(.vertical, 18)
                .background(.quaternary.opacity(0.5), in: RoundedRectangle(cornerRadius: 8))
            HStack {
                Button("Cancel") {
                    stop()
                    onCancel()
                }
                Button("Clear") {
                    stop()
                    onCapture([])
                }
            }
        }
        .padding(28)
        .frame(width: 420)
        .onAppear { start() }
        .onDisappear { stop() }
    }

    private func start() {
        stop()
        monitor = NSEvent.addLocalMonitorForEvents(matching: [.keyDown]) { event in
            guard let key = keyName(for: event) else { return nil }
            var keys: [String] = []
            let flags = event.modifierFlags
            if flags.contains(.command) { keys.append("command") }
            if flags.contains(.control) { keys.append("control") }
            if flags.contains(.option) { keys.append("option") }
            if flags.contains(.shift) { keys.append("shift") }
            // macOS often sets .function for hardware/layout reasons even when Fn was not intentionally held.
            // Keep Fn available for manual input, but do not add it during shortcut recording.
            keys.append(key)
            preview = displayKeyList(keys)
            DispatchQueue.main.asyncAfter(deadline: .now() + 0.12) {
                stop()
                onCapture(keys)
            }
            return nil
        }
    }

    private func stop() {
        if let monitor {
            NSEvent.removeMonitor(monitor)
        }
        monitor = nil
    }
}

private struct StatusPill: View {
    let text: String
    let active: Bool

    var body: some View {
        HStack(spacing: 6) {
            Circle()
                .fill(active ? Color.green : Color.gray)
                .frame(width: 8, height: 8)
            Text(text)
                .font(.caption)
                .lineLimit(1)
        }
        .padding(.horizontal, 10)
        .padding(.vertical, 5)
        .background(.quaternary, in: Capsule())
    }
}

private struct StatusDashboard: View {
    let running: Bool
    let deviceStatus: String
    let frontmostName: String
    let lockMode: String
    let effectiveMapping: String
    let inputAccessStatus: String
    let accessibilityStatus: String
    let inputCounts: [KnobInput: Int]
    let lastInput: KnobInput?
    let lastModifierMode: ModifierMode
    let pressSequenceCount: Int
    let rawEventCount: Int
    let decodedEventCount: Int
    let ignoredEventCount: Int
    let lastRawEvent: String
    let lastEvent: String

    var body: some View {
        VStack(spacing: 10) {
            HStack(spacing: 10) {
                DashboardTile(
                    title: "Device status",
                    value: running ? "Listening" : "Not listening",
                    detail: "\(deviceStatus) · \(inputAccessStatus) · \(accessibilityStatus)",
                    color: running ? .green : .gray
                )
                DashboardTile(
                    title: "Current app",
                    value: frontmostName,
                    detail: "In use: \(effectiveMapping)",
                    color: .blue
                )
                DashboardTile(
                    title: "Mapping mode",
                    value: lockMode,
                    detail: lockMode.hasPrefix("Auto") ? "Mappings follow the frontmost app"
                        : pressSequenceCount > 0 ? "Press count: \(pressSequenceCount)/3" : "Press three times quickly to switch for the current app",
                    color: .purple
                )
                DashboardTile(
                    title: "Last action",
                    value: lastEvent == "No events yet" ? "Waiting for knob input" : lastEvent,
                    detail: "Live action feedback",
                    color: .orange
                )
            }

            InputMonitorStrip(
                inputCounts: inputCounts,
                lastInput: lastInput,
                lastModifierMode: lastModifierMode,
                rawEventCount: rawEventCount,
                decodedEventCount: decodedEventCount,
                ignoredEventCount: ignoredEventCount,
                lastRawEvent: lastRawEvent
            )
        }
    }
}

private struct InputMonitorStrip: View {
    let inputCounts: [KnobInput: Int]
    let lastInput: KnobInput?
    let lastModifierMode: ModifierMode
    let rawEventCount: Int
    let decodedEventCount: Int
    let ignoredEventCount: Int
    let lastRawEvent: String

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack(spacing: 8) {
                Text("Input monitor")
                    .font(.caption.weight(.semibold))
                    .foregroundStyle(.secondary)
                Text("Raw \(rawEventCount)")
                Text("Recognized \(decodedEventCount)")
                Text("Ignored \(ignoredEventCount)")
                Text("Last \(lastInput.map { lastModifierMode.gestureTitle(for: $0) } ?? "none")")
                    .foregroundStyle(.secondary)
                Text(lastRawEvent)
                    .lineLimit(1)
                    .foregroundStyle(.secondary)
                Spacer(minLength: 0)
            }
            .font(.caption)

            HStack(spacing: 8) {
                ForEach(KnobInput.allCases) { input in
                    InputCounterPill(
                        input: input,
                        count: inputCounts[input, default: 0],
                        active: lastInput == input
                    )
                }
            }
        }
        .padding(10)
        .background(.quaternary.opacity(0.55), in: RoundedRectangle(cornerRadius: 8))
    }
}

private struct InputCounterPill: View {
    let input: KnobInput
    let count: Int
    let active: Bool

    var body: some View {
        HStack(spacing: 5) {
            Text(gestureSymbol(input))
            Text(input.title)
            Text("\(count)")
                .fontWeight(.semibold)
        }
        .font(.caption)
        .padding(.horizontal, 8)
        .padding(.vertical, 5)
        .background(active ? Color.accentColor.opacity(0.28) : Color.gray.opacity(0.14), in: Capsule())
    }
}

private struct DashboardTile: View {
    let title: String
    let value: String
    let detail: String
    let color: Color

    var body: some View {
        VStack(alignment: .leading, spacing: 5) {
            HStack(spacing: 6) {
                Circle()
                    .fill(color)
                    .frame(width: 8, height: 8)
                Text(title)
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
            Text(value)
                .font(.system(size: 13, weight: .semibold))
                .lineLimit(1)
            Text(detail)
                .font(.caption2)
                .foregroundStyle(.secondary)
                .lineLimit(1)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(10)
        .background(.quaternary.opacity(0.55), in: RoundedRectangle(cornerRadius: 8))
    }
}

private enum ActionOverlayStyle {
    case action
    case macVolume
}

private struct ActionOverlayPayload {
    let title: String
    let subtitle: String
    let systemImage: String
    let tint: Color
    let progress: Double?
    let ok: Bool
    var style: ActionOverlayStyle = .action
}

private struct ActionOverlayView: View {
    let payload: ActionOverlayPayload

    var body: some View {
        switch payload.style {
        case .action:
            actionOverlay
        case .macVolume:
            MacVolumeOverlayView(payload: payload)
        }
    }

    private var actionOverlay: some View {
        HStack(spacing: 14) {
            Image(systemName: payload.systemImage)
                .font(.system(size: 26, weight: .semibold))
                .foregroundStyle(payload.tint)
                .frame(width: 36, height: 36)

            VStack(alignment: .leading, spacing: 7) {
                Text(payload.title)
                    .font(.system(size: 17, weight: .semibold))
                    .lineLimit(1)
                Text(payload.subtitle)
                    .font(.system(size: 12, weight: .medium))
                    .foregroundStyle(.secondary)
                    .lineLimit(1)

                if let progress = payload.progress {
                    GeometryReader { proxy in
                        ZStack(alignment: .leading) {
                            Capsule()
                                .fill(.secondary.opacity(0.18))
                            Capsule()
                                .fill(payload.tint)
                                .frame(width: max(8, proxy.size.width * min(max(progress, 0), 1)))
                        }
                    }
                    .frame(height: 7)
                    .padding(.top, 1)
                }
            }
        }
        .padding(.horizontal, 18)
        .padding(.vertical, 16)
        .frame(width: 310, height: payload.progress == nil ? 88 : 108)
        .background(.regularMaterial, in: RoundedRectangle(cornerRadius: 18, style: .continuous))
        .overlay(
            RoundedRectangle(cornerRadius: 18, style: .continuous)
                .stroke(.white.opacity(0.16), lineWidth: 1)
        )
        .shadow(color: .black.opacity(0.22), radius: 22, x: 0, y: 10)
    }
}

private struct MacVolumeOverlayView: View {
    let payload: ActionOverlayPayload

    private let segmentCount = 16

    var body: some View {
        VStack(spacing: 0) {
            Spacer(minLength: 28)

            Image(systemName: payload.systemImage)
                .font(.system(size: 86, weight: .medium))
                .symbolRenderingMode(.hierarchical)
                .foregroundStyle(.white.opacity(0.82))
                .frame(height: 92)

            Spacer(minLength: 16)

            HStack(spacing: 3) {
                ForEach(0..<segmentCount, id: \.self) { index in
                    Rectangle()
                        .fill(index < filledSegments ? Color.white.opacity(0.82) : Color.black.opacity(0.34))
                        .frame(width: 8, height: 8)
                }
            }
            .accessibilityLabel(payload.title)

            Spacer(minLength: 24)
        }
        .frame(width: 228, height: 228)
        .background(
            LinearGradient(
                colors: [
                    Color.black.opacity(0.58),
                    Color.black.opacity(0.48)
                ],
                startPoint: .top,
                endPoint: .bottom
            ),
            in: RoundedRectangle(cornerRadius: 22, style: .continuous)
        )
        .overlay(
            RoundedRectangle(cornerRadius: 22, style: .continuous)
                .stroke(.white.opacity(0.08), lineWidth: 1)
        )
        .shadow(color: .black.opacity(0.28), radius: 28, x: 0, y: 14)
    }

    private var filledSegments: Int {
        let progress = min(max(payload.progress ?? 0, 0), 1)
        guard progress > 0 else { return 0 }
        return max(1, Int((progress * Double(segmentCount)).rounded(.up)))
    }
}

private final class OverlayPanel: NSPanel {
    override var canBecomeKey: Bool { false }
    override var canBecomeMain: Bool { false }
}

@MainActor
private final class OverlayPresenter {
    static let shared = OverlayPresenter()

    private var panel: OverlayPanel?
    private var hostingView: NSHostingView<ActionOverlayView>?
    private var hideWorkItem: DispatchWorkItem?

    func show(_ payload: ActionOverlayPayload) {
        hideWorkItem?.cancel()
        let panel = panel(for: payload)
        hostingView?.rootView = ActionOverlayView(payload: payload)
        position(panel, for: payload)

        panel.alphaValue = max(panel.alphaValue, 0.02)
        panel.orderFrontRegardless()
        NSAnimationContext.runAnimationGroup { context in
            context.duration = 0.12
            panel.animator().alphaValue = 1
        }

        let workItem = DispatchWorkItem { [weak self] in
            Task { @MainActor in self?.hide() }
        }
        hideWorkItem = workItem
        DispatchQueue.main.asyncAfter(deadline: .now() + displayDuration(for: payload), execute: workItem)
    }

    private func hide() {
        guard let panel else { return }
        NSAnimationContext.runAnimationGroup { context in
            context.duration = 0.18
            panel.animator().alphaValue = 0
        } completionHandler: {
            panel.orderOut(nil)
        }
    }

    private func panel(for payload: ActionOverlayPayload) -> OverlayPanel {
        if let panel { return panel }

        let size = overlaySize(for: payload)
        let panel = OverlayPanel(
            contentRect: NSRect(origin: .zero, size: size),
            styleMask: [.borderless, .nonactivatingPanel],
            backing: .buffered,
            defer: false
        )
        panel.backgroundColor = .clear
        panel.isOpaque = false
        panel.hasShadow = false
        panel.ignoresMouseEvents = true
        panel.hidesOnDeactivate = false
        panel.isReleasedWhenClosed = false
        panel.level = .statusBar
        panel.collectionBehavior = [.canJoinAllSpaces, .fullScreenAuxiliary, .ignoresCycle]

        let hostingView = NSHostingView(rootView: ActionOverlayView(payload: payload))
        hostingView.frame = NSRect(origin: .zero, size: size)
        hostingView.wantsLayer = true
        hostingView.layer?.backgroundColor = NSColor.clear.cgColor
        panel.contentView = hostingView

        self.panel = panel
        self.hostingView = hostingView
        return panel
    }

    private func position(_ panel: NSPanel, for payload: ActionOverlayPayload) {
        let size = overlaySize(for: payload)
        let screenFrame = (NSScreen.main ?? NSScreen.screens.first)?.visibleFrame ?? NSRect(x: 0, y: 0, width: 1440, height: 900)
        let origin: NSPoint
        switch payload.style {
        case .action:
            origin = NSPoint(
                x: screenFrame.maxX - size.width - 28,
                y: screenFrame.maxY - size.height - 34
            )
        case .macVolume:
            origin = NSPoint(
                x: screenFrame.midX - size.width / 2,
                y: screenFrame.midY - size.height / 2
            )
        }
        panel.setFrame(NSRect(origin: origin, size: size), display: true)
        hostingView?.frame = NSRect(origin: .zero, size: size)
    }

    private func overlaySize(for payload: ActionOverlayPayload) -> NSSize {
        switch payload.style {
        case .action:
            return NSSize(width: 310, height: payload.progress == nil ? 88 : 108)
        case .macVolume:
            return NSSize(width: 228, height: 228)
        }
    }

    private func displayDuration(for payload: ActionOverlayPayload) -> TimeInterval {
        switch payload.style {
        case .action:
            return 1.6
        case .macVolume:
            return 1.15
        }
    }
}

private struct ActionRunResult {
    let ok: Bool
    let detail: String?
    let overlay: ActionOverlayPayload?

    static func success(_ detail: String? = nil, overlay: ActionOverlayPayload? = nil) -> ActionRunResult {
        ActionRunResult(ok: true, detail: detail, overlay: overlay)
    }

    static func failure(_ detail: String? = nil, overlay: ActionOverlayPayload? = nil) -> ActionRunResult {
        ActionRunResult(ok: false, detail: detail, overlay: overlay)
    }
}

private enum ActionExecutor {
    static func run(_ action: ActionConfig) -> ActionRunResult {
        switch action.type {
        case "shortcut", "key":
            return sendKeys(action.keys ?? [])
        case "mouse":
            return click(button: action.button ?? "left", clicks: action.clicks ?? 1) ? .success() : .failure()
        case "scroll":
            return scroll(dx: action.dx ?? 0, dy: action.dy ?? 0) ? .success() : .failure()
        case "shell":
            return runShell(action.command) ? .success() : .failure()
        case "noop":
            return .success()
        default:
            return .failure("Unknown action type: \(action.type)")
        }
    }

    static func needsAccessibility(_ action: ActionConfig) -> Bool {
        switch action.type {
        case "shortcut", "key", "mouse", "scroll":
            if let keys = action.keys?.map({ $0.lowercased() }), keys.count == 1, SystemAudioController.canHandle(keys[0]) {
                return false
            }
            return true
        default:
            return false
        }
    }

    private static func sendKeys(_ keys: [String]) -> ActionRunResult {
        let normalized = keys.map { $0.lowercased() }
        if normalized.count == 1, SystemAudioController.canHandle(normalized[0]) {
            let result = SystemAudioController.handle(normalized[0])
            // Outputs such as DisplayPort / HDMI monitors have no software volume.
            // Fall back to the real media key so macOS, or a tool like
            // MonitorControl that drives the monitor over DDC, handles it.
            if !result.ok, let media = mediaKeyTypes[normalized[0]] {
                sendMediaKey(media)
                return .success("System \(displayKeyName(normalized[0])) key (output device has no software volume)")
            }
            return result
        }

        if normalized.count == 1, let media = mediaKeyTypes[normalized[0]] {
            sendMediaKey(media)
            return .success()
        }

        var flags = CGEventFlags()
        var keyCodes: [CGKeyCode] = []

        for key in normalized {
            if let flag = modifierFlags[key] {
                flags.insert(flag)
            } else if let code = keyCodesByName[key] {
                keyCodes.append(code)
            } else if let media = mediaKeyTypes[key] {
                sendMediaKey(media)
            } else {
                return .failure("Unrecognized key: \(key)")
            }
        }

        for keyCode in keyCodes {
            sendKey(keyCode, down: true, flags: flags)
            sendKey(keyCode, down: false, flags: flags)
        }
        return .success()
    }

    private static func sendKey(_ keyCode: CGKeyCode, down: Bool, flags: CGEventFlags) {
        guard let event = CGEvent(keyboardEventSource: nil, virtualKey: keyCode, keyDown: down) else { return }
        event.flags = flags
        event.post(tap: .cghidEventTap)
    }

    private static func sendMediaKey(_ keyType: Int) {
        postMediaKey(keyType, down: true)
        postMediaKey(keyType, down: false)
    }

    private static func postMediaKey(_ keyType: Int, down: Bool) {
        let flags = NSEvent.ModifierFlags(rawValue: UInt(down ? 0xA00 : 0xB00))
        let data1 = (keyType << 16) | ((down ? 0xA : 0xB) << 8)
        guard let event = NSEvent.otherEvent(
            with: .systemDefined,
            location: .zero,
            modifierFlags: flags,
            timestamp: 0,
            windowNumber: 0,
            context: nil,
            subtype: 8,
            data1: data1,
            data2: -1
        )?.cgEvent else { return }
        event.setIntegerValueField(.eventSourceUserData, value: NativeKeyBlocker.ownEventTag)
        event.post(tap: .cghidEventTap)
    }

    private static func click(button: String, clicks: Int) -> Bool {
        let mouseButton: CGMouseButton
        let downType: CGEventType
        let upType: CGEventType

        switch button.lowercased() {
        case "left":
            mouseButton = .left; downType = .leftMouseDown; upType = .leftMouseUp
        case "right":
            mouseButton = .right; downType = .rightMouseDown; upType = .rightMouseUp
        case "middle", "center":
            mouseButton = .center; downType = .otherMouseDown; upType = .otherMouseUp
        default:
            return false
        }

        let point = CGEvent(source: nil)?.location ?? .zero
        for _ in 0..<max(clicks, 1) {
            CGEvent(mouseEventSource: nil, mouseType: downType, mouseCursorPosition: point, mouseButton: mouseButton)?.post(tap: .cghidEventTap)
            CGEvent(mouseEventSource: nil, mouseType: upType, mouseCursorPosition: point, mouseButton: mouseButton)?.post(tap: .cghidEventTap)
        }
        return true
    }

    private static func scroll(dx: Int, dy: Int) -> Bool {
        guard let event = CGEvent(scrollWheelEvent2Source: nil, units: .line, wheelCount: 2, wheel1: Int32(dy), wheel2: Int32(dx), wheel3: 0) else {
            return false
        }
        event.post(tap: .cghidEventTap)
        return true
    }

    private static func runShell(_ command: String?) -> Bool {
        guard let command, !command.isEmpty else { return false }
        let process = Process()
        process.executableURL = URL(fileURLWithPath: "/bin/zsh")
        process.arguments = ["-lc", command]
        do {
            try process.run()
            process.waitUntilExit()
            return process.terminationStatus == 0
        } catch {
            return false
        }
    }
}

private let modifierFlags: [String: CGEventFlags] = [
    "command": .maskCommand, "cmd": .maskCommand,
    "control": .maskControl, "ctrl": .maskControl,
    "option": .maskAlternate, "opt": .maskAlternate, "alt": .maskAlternate,
    "shift": .maskShift, "fn": .maskSecondaryFn
]

private let mediaKeyTypes: [String: Int] = [
    "volume_up": 0, "volume_down": 1, "mute": 7,
    "brightness_up": 2, "brightness_down": 3,
    "play_pause": 16, "next_track": 17, "previous_track": 18
]

private enum SystemAudioController {
    private struct VolumeSnapshot {
        let value: Float32
        let source: String
    }

    private struct VolumeWrite {
        let ok: Bool
        let source: String
        let status: OSStatus
    }

    static func canHandle(_ key: String) -> Bool {
        ["volume_down", "volume_up", "mute"].contains(key)
    }

    static func handle(_ key: String) -> ActionRunResult {
        switch key {
        case "volume_down":
            return adjustVolume(by: -0.06)
        case "volume_up":
            return adjustVolume(by: 0.06)
        case "mute":
            return toggleMute()
        default:
            return .failure("Cannot handle system volume key: \(key)")
        }
    }

    private static func adjustVolume(by delta: Float32) -> ActionRunResult {
        guard let deviceID = defaultOutputDevice() else {
            return .failure("Default output device not found")
        }

        guard let before = currentVolume(deviceID: deviceID) else {
            return .failure("Cannot read default output device volume (device \(deviceID))")
        }

        let target = min(max(before.value + delta, 0), 1)
        let write = setVolume(deviceID: deviceID, target)
        let after = currentVolume(deviceID: deviceID)
        let beforeText = percent(before.value)
        let targetText = percent(target)
        let afterText = after.map { percent($0.value) } ?? "Unknown"
        let source = after?.source ?? write.source
        let detail = "Volume \(beforeText) -> \(afterText) (target \(targetText), \(source), status \(write.status))"
        let overlayValue = Double(after?.value ?? target)
        let overlay = ActionOverlayPayload(
            title: "Volume \(afterText)",
            subtitle: delta > 0 ? "Volume up · \(source)" : "Volume down · \(source)",
            systemImage: speakerImage(for: Float32(overlayValue)),
            tint: write.ok ? .accentColor : .red,
            progress: overlayValue,
            ok: write.ok,
            style: .macVolume
        )
        return write.ok ? .success(detail, overlay: overlay) : .failure(detail, overlay: overlay)
    }

    private static func currentVolume(deviceID: AudioDeviceID) -> VolumeSnapshot? {
        var address = volumeAddress()
        var volume = Float32(0)
        var size = UInt32(MemoryLayout<Float32>.size)
        let status = AudioObjectGetPropertyData(deviceID, &address, 0, nil, &size, &volume)
        if status == noErr {
            return VolumeSnapshot(value: volume, source: "VirtualMain")
        }

        var channelValues: [Float32] = []
        for element in [AudioObjectPropertyElement(1), AudioObjectPropertyElement(2)] {
            var channelAddress = scalarVolumeAddress(element: element)
            guard AudioObjectHasProperty(deviceID, &channelAddress) else { continue }
            var channelVolume = Float32(0)
            var channelSize = UInt32(MemoryLayout<Float32>.size)
            let channelStatus = AudioObjectGetPropertyData(deviceID, &channelAddress, 0, nil, &channelSize, &channelVolume)
            if channelStatus == noErr {
                channelValues.append(channelVolume)
            }
        }

        guard !channelValues.isEmpty else { return nil }
        let average = channelValues.reduce(Float32(0), +) / Float32(channelValues.count)
        return VolumeSnapshot(value: average, source: "Channel volume")
    }

    private static func setVolume(deviceID: AudioDeviceID, _ volume: Float32) -> VolumeWrite {
        var address = volumeAddress()
        var newVolume = volume
        let size = UInt32(MemoryLayout<Float32>.size)
        if AudioObjectHasProperty(deviceID, &address) {
            var settable = DarwinBoolean(false)
            var settableAddress = address
            let settableStatus = AudioObjectIsPropertySettable(deviceID, &settableAddress, &settable)
            if settableStatus == noErr, settable.boolValue {
                let status = AudioObjectSetPropertyData(deviceID, &address, 0, nil, size, &newVolume)
                if status == noErr {
                    return VolumeWrite(ok: true, source: "VirtualMain", status: status)
                }
            }
        }

        var statuses: [OSStatus] = []
        var successes = 0
        for element in [AudioObjectPropertyElement(1), AudioObjectPropertyElement(2)] {
            var channelAddress = scalarVolumeAddress(element: element)
            guard AudioObjectHasProperty(deviceID, &channelAddress) else { continue }
            var channelSettable = DarwinBoolean(false)
            var settableAddress = channelAddress
            let settableStatus = AudioObjectIsPropertySettable(deviceID, &settableAddress, &channelSettable)
            guard settableStatus == noErr, channelSettable.boolValue else {
                statuses.append(settableStatus)
                continue
            }
            var channelVolume = volume
            let status = AudioObjectSetPropertyData(deviceID, &channelAddress, 0, nil, size, &channelVolume)
            statuses.append(status)
            if status == noErr {
                successes += 1
            }
        }

        return VolumeWrite(
            ok: successes > 0,
            source: successes > 0 ? "Channel volume" : "No writable volume property",
            status: statuses.first ?? -1
        )
    }

    private static func toggleMute() -> ActionRunResult {
        guard let deviceID = defaultOutputDevice() else {
            return .failure("Default output device not found")
        }
        var address = AudioObjectPropertyAddress(
            mSelector: kAudioDevicePropertyMute,
            mScope: kAudioDevicePropertyScopeOutput,
            mElement: kAudioObjectPropertyElementMain
        )
        var muted = UInt32(0)
        var size = UInt32(MemoryLayout<UInt32>.size)
        let readStatus = AudioObjectGetPropertyData(deviceID, &address, 0, nil, &size, &muted)
        guard readStatus == noErr else {
            let before = currentVolume(deviceID: deviceID)
            let write = setVolume(deviceID: deviceID, 0)
            let after = currentVolume(deviceID: deviceID)
            let detail = "Mute fallback: volume \(before.map { percent($0.value) } ?? "Unknown") -> \(after.map { percent($0.value) } ?? "Unknown") (status \(write.status))"
            let overlay = ActionOverlayPayload(
                title: "Mute",
                subtitle: "Volume \(after.map { percent($0.value) } ?? "Unknown")",
                systemImage: "speaker.slash.fill",
                tint: write.ok ? .accentColor : .red,
                progress: after.map { Double($0.value) },
                ok: write.ok,
                style: .macVolume
            )
            return write.ok ? .success(detail, overlay: overlay) : .failure(detail, overlay: overlay)
        }

        var newMuted = muted == 0 ? UInt32(1) : UInt32(0)
        let status = AudioObjectSetPropertyData(deviceID, &address, 0, nil, size, &newMuted)
        let detail = muted == 0 ? "Mute on (status \(status))" : "Mute off (status \(status))"
        let ok = status == noErr
        let overlay = ActionOverlayPayload(
            title: muted == 0 ? "Mute" : "Unmute",
            subtitle: "Default output device",
            systemImage: muted == 0 ? "speaker.slash.fill" : "speaker.wave.2.fill",
            tint: ok ? .accentColor : .red,
            progress: muted == 0 ? 0 : currentVolume(deviceID: deviceID).map { Double($0.value) },
            ok: ok,
            style: .macVolume
        )
        return ok ? .success(detail, overlay: overlay) : .failure(detail, overlay: overlay)
    }

    private static func defaultOutputDevice() -> AudioDeviceID? {
        var deviceID = AudioDeviceID(0)
        var size = UInt32(MemoryLayout<AudioDeviceID>.size)
        var address = AudioObjectPropertyAddress(
            mSelector: kAudioHardwarePropertyDefaultOutputDevice,
            mScope: kAudioObjectPropertyScopeGlobal,
            mElement: kAudioObjectPropertyElementMain
        )
        let status = AudioObjectGetPropertyData(
            AudioObjectID(kAudioObjectSystemObject),
            &address,
            0,
            nil,
            &size,
            &deviceID
        )
        return status == noErr ? deviceID : nil
    }

    private static func volumeAddress() -> AudioObjectPropertyAddress {
        AudioObjectPropertyAddress(
            mSelector: kAudioHardwareServiceDeviceProperty_VirtualMainVolume,
            mScope: kAudioDevicePropertyScopeOutput,
            mElement: kAudioObjectPropertyElementMain
        )
    }

    private static func scalarVolumeAddress(element: AudioObjectPropertyElement) -> AudioObjectPropertyAddress {
        AudioObjectPropertyAddress(
            mSelector: kAudioDevicePropertyVolumeScalar,
            mScope: kAudioDevicePropertyScopeOutput,
            mElement: element
        )
    }

    private static func percent(_ value: Float32) -> String {
        "\(Int(round(value * 100)))%"
    }

    private static func speakerImage(for value: Float32) -> String {
        if value <= 0.01 {
            return "speaker.slash.fill"
        }
        if value < 0.36 {
            return "speaker.wave.1.fill"
        }
        return "speaker.wave.3.fill"
    }
}

private let keyCodesByName: [String: CGKeyCode] = [
    "a": 0, "s": 1, "d": 2, "f": 3, "h": 4, "g": 5, "z": 6, "x": 7,
    "c": 8, "v": 9, "b": 11, "q": 12, "w": 13, "e": 14, "r": 15,
    "y": 16, "t": 17, "1": 18, "2": 19, "3": 20, "4": 21, "6": 22,
    "5": 23, "=": 24, "9": 25, "7": 26, "-": 27, "8": 28, "0": 29,
    "]": 30, "o": 31, "u": 32, "[": 33, "i": 34, "p": 35, "return": 36,
    "enter": 36, "l": 37, "j": 38, "'": 39, "k": 40, ";": 41, "\\": 42,
    ",": 43, "/": 44, "n": 45, "m": 46, ".": 47, "tab": 48, "space": 49,
    "`": 50, "delete": 51, "backspace": 51, "escape": 53, "esc": 53,
    "left": 123, "right": 124, "down": 125, "up": 126,
    "f1": 122, "f2": 120, "f3": 99, "f4": 118, "f5": 96, "f6": 97,
    "f7": 98, "f8": 100, "f9": 101, "f10": 109, "f11": 103, "f12": 111
]

private let keyNameByCode: [CGKeyCode: String] = {
    var result: [CGKeyCode: String] = [:]
    for (name, code) in keyCodesByName {
        if result[code] == nil || name.count > result[code]!.count {
            result[code] = name
        }
    }
    result[36] = "return"
    result[49] = "space"
    result[53] = "escape"
    return result
}()

// The knob's hold + turn can be reassigned in the vendor's configuration app to
// F13 (hold + turn right) and Shift + F13 (hold + turn left). Its default
// brightness codes make Windows show its own brightness flyout whatever the
// Windows app does, so the knob is set up that way; this side reads the same
// keys. Only ever touched on the main run loop, where the HID callback runs.
private var knobShiftDown = false

private func knobInput(page: UInt32, usage: UInt32, intValue: Int) -> KnobInput? {
    if page == 0x0007, usage == 0x00e1 || usage == 0x00e5 { // left / right Shift of the knob
        knobShiftDown = intValue != 0
        return nil
    }
    guard intValue != 0 else { return nil }

    switch page {
    case 0x0007:
        return usage == 0x0068 ? (knobShiftDown ? .pressRotateLeft : .pressRotateRight) : nil // F13
    case 0x000c:
        switch usage {
        case 0x00e9: return .rotateRight
        case 0x00ea: return .rotateLeft
        case 0x00e2: return .press
        case 0x006f: return .pressRotateRight
        case 0x0070: return .pressRotateLeft
        default: return nil
        }
    case 0x0001:
        switch usage {
        case 0x0037, 0x0038:
            return intValue > 0 ? .rotateRight : .rotateLeft
        default:
            return nil
        }
    case 0x0009:
        return .press
    default:
        return nil
    }
}

private func defaultActions() -> [String: ActionConfig] {
    var actions: [String: ActionConfig] = [:]
    for input in KnobInput.allCases {
        actions[input.rawValue] = ActionConfig.empty()
    }
    return actions
}

private func globalDefaultActions() -> [String: ActionConfig] {
    templateActions(.media)
}

private func templateActions(_ template: ActionTemplate) -> [String: ActionConfig] {
    switch template {
    case .browser:
        return [
            KnobInput.rotateLeft.rawValue: shortcut("Previous tab", ["control", "shift", "tab"]),
            KnobInput.rotateRight.rawValue: shortcut("Next tab", ["control", "tab"]),
            KnobInput.press.rawValue: shortcut("Reload page", ["command", "r"]),
            KnobInput.pressRotateLeft.rawValue: shortcut("Back", ["command", "["]),
            KnobInput.pressRotateRight.rawValue: shortcut("Forward", ["command", "]"])
        ]
    case .coding:
        return [
            KnobInput.rotateLeft.rawValue: shortcut("Previous editor", ["command", "shift", "["]),
            KnobInput.rotateRight.rawValue: shortcut("Next editor", ["command", "shift", "]"]),
            KnobInput.press.rawValue: shortcut("Command palette", ["command", "shift", "p"]),
            KnobInput.pressRotateLeft.rawValue: shortcut("Previous problem", ["f8"]),
            KnobInput.pressRotateRight.rawValue: shortcut("Next problem", ["shift", "f8"])
        ]
    case .media:
        return [
            KnobInput.rotateLeft.rawValue: ActionConfig(type: "key", description: "Volume Down", keys: ["volume_down"], button: nil, clicks: nil, dx: nil, dy: nil, command: nil),
            KnobInput.rotateRight.rawValue: ActionConfig(type: "key", description: "Volume Up", keys: ["volume_up"], button: nil, clicks: nil, dx: nil, dy: nil, command: nil),
            KnobInput.press.rawValue: ActionConfig(type: "key", description: "Play / Pause", keys: ["play_pause"], button: nil, clicks: nil, dx: nil, dy: nil, command: nil),
            KnobInput.pressRotateLeft.rawValue: shortcut("Previous Track", ["previous_track"]),
            KnobInput.pressRotateRight.rawValue: shortcut("Next Track", ["next_track"])
        ]
    case .design:
        return [
            KnobInput.rotateLeft.rawValue: shortcut("Smaller brush", ["["]),
            KnobInput.rotateRight.rawValue: shortcut("Larger brush", ["]"]),
            KnobInput.press.rawValue: shortcut("Brush tool", ["b"]),
            KnobInput.pressRotateLeft.rawValue: shortcut("Lower hardness", ["shift", "["]),
            KnobInput.pressRotateRight.rawValue: shortcut("Higher hardness", ["shift", "]"])
        ]
    case .editing:
        return [
            KnobInput.rotateLeft.rawValue: shortcut("Previous frame", ["left"]),
            KnobInput.rotateRight.rawValue: shortcut("Next frame", ["right"]),
            KnobInput.press.rawValue: shortcut("Play / Pause", ["space"]),
            KnobInput.pressRotateLeft.rawValue: shortcut("Big step back", ["shift", "left"]),
            KnobInput.pressRotateRight.rawValue: shortcut("Big step forward", ["shift", "right"])
        ]
    }
}

private func shortcut(_ description: String, _ keys: [String]) -> ActionConfig {
    ActionConfig(type: "shortcut", description: description, keys: keys, button: nil, clicks: nil, dx: nil, dy: nil, command: nil)
}

private func optionalString(_ binding: Binding<String?>) -> Binding<String> {
    Binding(
        get: { binding.wrappedValue ?? "" },
        set: { binding.wrappedValue = $0 }
    )
}

private func optionalInt(_ binding: Binding<Int?>, defaultValue: Int) -> Binding<Int> {
    Binding(
        get: { binding.wrappedValue ?? defaultValue },
        set: { binding.wrappedValue = $0 }
    )
}

private func deviceName(_ device: IOHIDDevice) -> String {
    let product = property(device, kIOHIDProductKey) ?? "unknown"
    let manufacturer = property(device, kIOHIDManufacturerKey) ?? "unknown"
    let serial = property(device, kIOHIDSerialNumberKey) ?? "unknown"
    return "\(product) / \(manufacturer) / \(serial)"
}

private func property(_ device: IOHIDDevice, _ key: String) -> String? {
    IOHIDDeviceGetProperty(device, key as CFString).map { "\($0)" }
}

private func intProperty(_ device: IOHIDDevice, _ key: String) -> Int? {
    IOHIDDeviceGetProperty(device, key as CFString).flatMap { $0 as? NSNumber }?.intValue
}

private func diagnosticHIDDevices() -> [String] {
    let manager = IOHIDManagerCreate(kCFAllocatorDefault, IOOptionBits(kIOHIDOptionsTypeNone))
    IOHIDManagerSetDeviceMatching(manager, [:] as CFDictionary)
    let openResult = IOHIDManagerOpen(manager, IOOptionBits(kIOHIDOptionsTypeNone))
    defer { IOHIDManagerClose(manager, IOOptionBits(kIOHIDOptionsTypeNone)) }

    guard openResult == kIOReturnSuccess,
          let devices = IOHIDManagerCopyDevices(manager) as? Set<IOHIDDevice> else {
        return ["HID enumeration failed: 0x\(hex(UInt32(bitPattern: openResult), width: 8))"]
    }

    return devices.compactMap { device in
        let product = property(device, kIOHIDProductKey) ?? "unknown"
        let manufacturer = property(device, kIOHIDManufacturerKey) ?? "unknown"
        let serial = property(device, kIOHIDSerialNumberKey) ?? "unknown"
        let transport = property(device, kIOHIDTransportKey) ?? "unknown"
        let vendor = intProperty(device, kIOHIDVendorIDKey)
        let productNumber = intProperty(device, kIOHIDProductIDKey)
        let usagePage = intProperty(device, kIOHIDPrimaryUsagePageKey)
        let usage = intProperty(device, kIOHIDPrimaryUsageKey)
        let combined = "\(product) \(manufacturer) \(serial)".lowercased()
        let isTarget = vendor == vendorID && productNumber == productID
        let isSuspicious = combined.contains("anticater") || combined.contains("cxkj")

        guard isTarget || isSuspicious else { return nil }

        let vendorText = vendor.map { "0x\(hex($0, width: 4))" } ?? "unknown"
        let productText = productNumber.map { "0x\(hex($0, width: 4))" } ?? "unknown"
        let usagePageText = usagePage.map { "0x\(hex($0, width: 4))" } ?? "unknown"
        let usageText = usage.map { "0x\(hex($0, width: 4))" } ?? "unknown"
        return "product=\(product) manufacturer=\(manufacturer) serial=\(serial) transport=\(transport) vendor=\(vendorText) productID=\(productText) usagePage=\(usagePageText) usage=\(usageText)"
    }
    .sorted()
}

private func appName(bundle: Bundle, url: URL) -> String {
    let localizedName = bundle.object(forInfoDictionaryKey: "CFBundleDisplayName") as? String
    let bundleName = bundle.object(forInfoDictionaryKey: "CFBundleName") as? String
    return localizedName ?? bundleName ?? url.deletingPathExtension().lastPathComponent
}

private func isVisibleApplication(bundle: Bundle) -> Bool {
    let backgroundOnly = bundle.object(forInfoDictionaryKey: "LSBackgroundOnly") as? Bool ?? false
    let uiElement = bundle.object(forInfoDictionaryKey: "LSUIElement") as? Bool ?? false
    return !backgroundOnly && !uiElement
}

private func appInfoFromBundleID(_ bundleID: String) -> AppInfo? {
    guard let url = NSWorkspace.shared.urlForApplication(withBundleIdentifier: bundleID),
          let bundle = Bundle(url: url) else {
        return nil
    }

    return AppInfo(
        bundleID: bundleID,
        name: appName(bundle: bundle, url: url),
        path: url.path,
        isRunning: NSWorkspace.shared.runningApplications.contains {
            $0.bundleIdentifier == bundleID && $0.activationPolicy == .regular
        }
    )
}

private func displayKeyList(_ keys: [String]) -> String {
    keys.map { displayKeyName($0) }.joined(separator: " + ")
}

private func actionSummary(_ action: ActionConfig) -> String {
    let desc = action.description?.trimmingCharacters(in: .whitespacesAndNewlines)
    if let desc, !desc.isEmpty {
        return desc
    }

    switch action.type {
    case "shortcut", "key":
        let keys = action.keys ?? []
        return keys.isEmpty ? "No key set" : displayKeyList(keys)
    case "mouse":
        return "Mouse \(displayMouseButton(action.button ?? "left")) · clicks: \(action.clicks ?? 1)"
    case "scroll":
        return "Horizontal \(action.dx ?? 0), vertical \(action.dy ?? 0)"
    case "shell":
        return action.command?.isEmpty == false ? "Run command" : "No command set"
    case "noop":
        return "Ignore this gesture"
    default:
        return "Unrecognized action"
    }
}

private func gestureSymbol(_ input: KnobInput) -> String {
    switch input {
    case .rotateLeft: return "↺"
    case .rotateRight: return "↻"
    case .press: return "●"
    case .pressRotateLeft: return "●↺"
    case .pressRotateRight: return "●↻"
    }
}

private func overlaySystemImage(for input: KnobInput) -> String {
    switch input {
    case .rotateLeft: return "arrow.counterclockwise"
    case .rotateRight: return "arrow.clockwise"
    case .press: return "circle.fill"
    case .pressRotateLeft: return "arrow.counterclockwise.circle.fill"
    case .pressRotateRight: return "arrow.clockwise.circle.fill"
    }
}

private func displayMouseButton(_ value: String) -> String {
    switch value.lowercased() {
    case "right": return "Right"
    case "middle", "center": return "Middle"
    default: return "Left"
    }
}

private func parseKeyList(_ text: String) -> [String] {
    text
        .replacingOccurrences(of: "+", with: ",")
        .split(separator: ",")
        .map { $0.trimmingCharacters(in: .whitespacesAndNewlines) }
        .compactMap { tokenFromDisplayName($0) }
}

private func displayKeyName(_ token: String) -> String {
    switch token.lowercased() {
    case "command", "cmd": return "⌘"
    case "control", "ctrl": return "⌃"
    case "option", "opt", "alt": return "⌥"
    case "shift": return "⇧"
    case "fn": return "fn"
    case "volume_down": return "Volume Down"
    case "volume_up": return "Volume Up"
    case "brightness_down": return "Brightness Down"
    case "brightness_up": return "Brightness Up"
    case "play_pause": return "Play/Pause"
    case "previous_track": return "Previous Track"
    case "next_track": return "Next Track"
    case "mute": return "Mute"
    case "return": return "Return"
    case "escape", "esc": return "Esc"
    case "left": return "Left"
    case "right": return "Right"
    case "up": return "Up"
    case "down": return "Down"
    case "space": return "Space"
    default:
        return token.count == 1 ? token.uppercased() : token
    }
}

private func tokenFromDisplayName(_ value: String) -> String? {
    let trimmed = value.trimmingCharacters(in: .whitespacesAndNewlines)
    guard !trimmed.isEmpty else { return nil }

    switch trimmed.lowercased() {
    case "⌘", "command", "cmd", "命令": return "command"
    case "⌃", "control", "ctrl", "控制": return "control"
    case "⌥", "option", "opt", "alt", "选项": return "option"
    case "⇧", "shift", "换挡", "上档": return "shift"
    case "fn", "功能": return "fn"
    case "降低音量": return "volume_down"
    case "提高音量": return "volume_up"
    case "播放/暂停", "播放", "暂停": return "play_pause"
    case "上一首", "上一个媒体项目": return "previous_track"
    case "下一首", "下一个媒体项目": return "next_track"
    case "静音": return "mute"
    case "回车", "return", "enter": return "return"
    case "退出", "escape", "esc": return "escape"
    case "左方向", "left": return "left"
    case "右方向", "right": return "right"
    case "上方向", "up": return "up"
    case "下方向", "down": return "down"
    case "空格", "space": return "space"
    case "volume down": return "volume_down"
    case "volume up": return "volume_up"
    case "brightness down": return "brightness_down"
    case "brightness up": return "brightness_up"
    case "play/pause": return "play_pause"
    case "previous track": return "previous_track"
    case "next track": return "next_track"
    default:
        return trimmed.lowercased()
    }
}

private func keyName(for event: NSEvent) -> String? {
    if let named = keyNameByCode[CGKeyCode(event.keyCode)] {
        return named
    }

    guard let characters = event.charactersIgnoringModifiers?.lowercased(),
          let first = characters.first else {
        return nil
    }

    return String(first)
}

private func clock() -> String {
    let formatter = DateFormatter()
    formatter.dateFormat = "HH:mm:ss"
    return formatter.string(from: Date())
}

private func hex<T: FixedWidthInteger>(_ value: T, width: Int) -> String {
    String(format: "%0\(width)x", UInt64(value))
}

private func userConfigURL() -> URL {
    let fm = FileManager.default
    let base = fm.urls(for: .applicationSupportDirectory, in: .userDomainMask).first
        ?? URL(fileURLWithPath: NSHomeDirectory()).appendingPathComponent("Library/Application Support")
    return base
        .appendingPathComponent("ANTICATER Knob Control", isDirectory: true)
        .appendingPathComponent("app-mapping.json")
}

private func findProjectRoot() -> URL {
    let fm = FileManager.default
    var candidates: [URL] = []
    candidates.append(URL(fileURLWithPath: fm.currentDirectoryPath))

    if let env = ProcessInfo.processInfo.environment["ANTICATER_KNOB_HOME"] {
        candidates.append(URL(fileURLWithPath: env))
    }

    let executable = URL(fileURLWithPath: CommandLine.arguments[0]).standardizedFileURL
    var cursor = executable.deletingLastPathComponent()
    for _ in 0..<8 {
        candidates.append(cursor)
        cursor.deleteLastPathComponent()
    }

    for candidate in candidates {
        if fm.fileExists(atPath: candidate.appendingPathComponent("config/app-mapping.json").path)
            || fm.fileExists(atPath: candidate.appendingPathComponent("config/app-mapping.example.json").path) {
            return candidate
        }
    }

    return URL(fileURLWithPath: fm.currentDirectoryPath)
}
