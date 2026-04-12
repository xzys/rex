import os

private let subsystem = "com.rex.companion"

struct Log {
    private let log: OSLog
    private let category: String

    init(category: String) {
        self.category = category
        self.log = OSLog(subsystem: subsystem, category: category)
    }

    func info(_ message: String) {
        #if DEBUG
        os_log("%{public}s", log: log, type: .default, message)
        #else
        os_log("%s", log: log, type: .info, message)
        #endif
    }

    func debug(_ message: String) {
        #if DEBUG
        os_log("%{public}s", log: log, type: .default, message)
        #else
        os_log("%s", log: log, type: .debug, message)
        #endif
    }

    func error(_ message: String) {
        #if DEBUG
        os_log("%{public}s", log: log, type: .error, message)
        #else
        os_log("%s", log: log, type: .error, message)
        #endif
    }
}
