import Foundation
import os

private let subsystem = "com.rex.companion"

struct Log {
    private let log: OSLog

    init(category: String) {
        self.log = OSLog(subsystem: subsystem, category: category)
    }

    func info(_ message: String) {
        #if DEBUG
        os_log("%{public}@", log: log, type: .info, message as NSString)
        #else
        os_log("%@", log: log, type: .info, message as NSString)
        #endif
    }

    func debug(_ message: String) {
        #if DEBUG
        os_log("%{public}@", log: log, type: .debug, message as NSString)
        #else
        os_log("%@", log: log, type: .debug, message as NSString)
        #endif
    }

    func error(_ message: String) {
        #if DEBUG
        os_log("%{public}@", log: log, type: .error, message as NSString)
        #else
        os_log("%@", log: log, type: .error, message as NSString)
        #endif
    }
}
