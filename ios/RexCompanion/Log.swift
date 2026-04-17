import Foundation
import os

private let subsystem = "com.rex.companion"

struct Log {
    private let log: OSLog
    private let category: String

    init(category: String) {
        self.log = OSLog(subsystem: subsystem, category: category)
        self.category = category
    }

    func info(_ message: String) {
        #if DEBUG
        os_log("%{public}@", log: log, type: .info, message as NSString)
        print("[\(category)] INFO \(message)")
        #else
        os_log("%@", log: log, type: .info, message as NSString)
        #endif
    }

    func debug(_ message: String) {
        #if DEBUG
        os_log("%{public}@", log: log, type: .debug, message as NSString)
        print("[\(category)] DEBUG \(message)")
        #else
        os_log("%@", log: log, type: .debug, message as NSString)
        #endif
    }

    func error(_ message: String) {
        #if DEBUG
        os_log("%{public}@", log: log, type: .error, message as NSString)
        print("[\(category)] ERROR \(message)")
        #else
        os_log("%@", log: log, type: .error, message as NSString)
        #endif
    }
}
