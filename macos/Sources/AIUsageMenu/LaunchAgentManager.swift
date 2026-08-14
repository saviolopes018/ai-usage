import Foundation

struct ProcessResult: Equatable {
    let status: Int32
    let output: String
}

protocol ProcessExecuting {
    func run(_ executable: String, arguments: [String]) throws -> ProcessResult
}

struct SystemProcessExecutor: ProcessExecuting {
    func run(_ executable: String, arguments: [String]) throws -> ProcessResult {
        let process = Process()
        let pipe = Pipe()
        process.executableURL = URL(fileURLWithPath: executable)
        process.arguments = arguments
        process.standardOutput = pipe
        process.standardError = pipe
        try process.run()
        process.waitUntilExit()
        let data = pipe.fileHandleForReading.readDataToEndOfFile()
        return ProcessResult(status: process.terminationStatus, output: String(decoding: data, as: UTF8.self))
    }
}

enum LaunchAgentState: Equatable {
    case running, installed, missing
}

struct LaunchAgentManager {
    static let label = "com.saviolopes.ai-usage-monitor"
    var executor: ProcessExecuting = SystemProcessExecutor()
    var userID: UInt32 = getuid()
    var homeDirectory: URL = FileManager.default.homeDirectoryForCurrentUser

    var serviceTarget: String { "gui/\(userID)/\(Self.label)" }
    var binaryURL: URL { homeDirectory.appendingPathComponent(".local/bin/usage-agent") }
    var installCommand: String { "\(binaryURL.path) install-service" }

    func status() -> LaunchAgentState {
        guard FileManager.default.fileExists(atPath: binaryURL.path) else { return .missing }
        guard let result = try? executor.run("/bin/launchctl", arguments: ["print", serviceTarget]), result.status == 0 else {
            return .installed
        }
        return result.output.contains("state = running") ? .running : .installed
    }

    func start() throws {
        guard status() != .missing else { throw CocoaError(.fileNoSuchFile) }
        let result = try executor.run("/bin/launchctl", arguments: ["kickstart", "-k", serviceTarget])
        guard result.status == 0 else {
            throw NSError(domain: "AIUsageMenu.LaunchAgent", code: Int(result.status), userInfo: [NSLocalizedDescriptionKey: result.output])
        }
    }
}
