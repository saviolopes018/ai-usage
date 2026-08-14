import Foundation
import XCTest
@testable import AIUsageMenu

final class MockURLProtocol: URLProtocol {
    static var handler: ((URLRequest) throws -> (HTTPURLResponse, Data))?

    override class func canInit(with request: URLRequest) -> Bool { true }
    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }
    override func startLoading() {
        do {
            let (response, data) = try Self.handler!(request)
            client?.urlProtocol(self, didReceive: response, cacheStoragePolicy: .notAllowed)
            client?.urlProtocol(self, didLoad: data)
            client?.urlProtocolDidFinishLoading(self)
        } catch {
            client?.urlProtocol(self, didFailWithError: error)
        }
    }
    override func stopLoading() {}
}

final class AgentClientTests: XCTestCase {
    private var client: AgentClient!
    private let config = AgentConfiguration(token: "secret", port: 9876)

    override func setUp() {
        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [MockURLProtocol.self]
        client = AgentClient(configuration: configuration)
    }

    override func tearDown() {
        MockURLProtocol.handler = nil
        client.disconnect()
    }

    func testFetchesStateWithBearerToken() async throws {
        MockURLProtocol.handler = { request in
            XCTAssertEqual(request.url?.absoluteString, "http://127.0.0.1:9876/state")
            XCTAssertEqual(request.value(forHTTPHeaderField: "Authorization"), "Bearer secret")
            let response = HTTPURLResponse(url: request.url!, statusCode: 200, httpVersion: nil, headerFields: nil)!
            let body = Data(#"{"protocolVersion":1,"agentVersion":"1.3.0","capabilities":[],"device":"Mac","online":true,"updatedAt":"2026-08-14T12:00:00Z","providers":[]}"#.utf8)
            return (response, body)
        }

        let snapshot = try await client.fetchState(configuration: config)
        XCTAssertEqual(snapshot.device, "Mac")
    }

    func testMapsUnauthorizedResponse() async {
        MockURLProtocol.handler = { request in
            (HTTPURLResponse(url: request.url!, statusCode: 401, httpVersion: nil, headerFields: nil)!, Data())
        }

        do {
            _ = try await client.fetchState(configuration: config)
            XCTFail("Expected unauthorized")
        } catch {
            XCTAssertEqual(error as? AgentClientError, .unauthorized)
        }
    }

    func testMapsUnavailableServerResponse() async {
        MockURLProtocol.handler = { request in
            (HTTPURLResponse(url: request.url!, statusCode: 503, httpVersion: nil, headerFields: nil)!, Data())
        }

        do {
            _ = try await client.fetchState(configuration: config)
            XCTFail("Expected server error")
        } catch {
            XCTAssertEqual(error as? AgentClientError, .server(503))
        }
    }

    func testRefreshUsesProviderEndpoint() async throws {
        MockURLProtocol.handler = { request in
            XCTAssertEqual(request.httpMethod, "POST")
            XCTAssertEqual(request.url?.path, "/codex/refresh")
            return (HTTPURLResponse(url: request.url!, statusCode: 202, httpVersion: nil, headerFields: nil)!, Data())
        }
        try await client.refresh(provider: "codex", configuration: config)
    }
}
