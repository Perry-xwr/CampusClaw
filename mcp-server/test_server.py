import unittest
import json
import os
import threading
import urllib.error
import urllib.request
from http.server import ThreadingHTTPServer
from unittest.mock import patch

from server import Handler, dispatch


class MCPDispatchTests(unittest.TestCase):
    def call(self, name, arguments):
        return dispatch({"jsonrpc": "2.0", "id": "test-1", "method": "tools/call", "params": {"name": name, "arguments": arguments}})

    def test_initialize_and_discovery(self):
        initialized = dispatch({"jsonrpc": "2.0", "id": 1, "method": "initialize"})
        self.assertEqual(initialized["result"]["protocolVersion"], "2024-11-05")
        tools = dispatch({"jsonrpc": "2.0", "id": 2, "method": "tools/list"})["result"]["tools"]
        self.assertEqual({tool["name"] for tool in tools}, {"formula_hint", "unit_convert"})
        self.assertTrue(all(tool["inputSchema"]["type"] == "object" for tool in tools))

    def test_formula_hint_is_guidance_only(self):
        result = self.call("formula_hint", {"topic": "speed"})["result"]["content"][0]["text"]
        self.assertIn("解题线索", result)
        self.assertNotIn("= 具体数值", result)

    def test_unit_convert(self):
        result = self.call("unit_convert", {"value": 1.5, "from_unit": "km", "to_unit": "m"})
        self.assertEqual(result["result"]["content"][0]["text"], "1.5 km = 1500 m")

    def test_rejects_invalid_units_and_unknown_methods(self):
        invalid = self.call("unit_convert", {"value": 1, "from_unit": "kg", "to_unit": "m"})
        self.assertEqual(invalid["error"]["code"], -32602)
        unknown = dispatch({"jsonrpc": "2.0", "id": 3, "method": "made/up"})
        self.assertEqual(unknown["error"]["code"], -32601)

    def test_http_authentication_and_jsonrpc(self):
        with patch.dict(os.environ, {"MCP_AUTH_TOKEN": "test-only-token"}):
            httpd = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
            thread = threading.Thread(target=httpd.serve_forever, daemon=True)
            thread.start()
            try:
                url = f"http://127.0.0.1:{httpd.server_port}/mcp"
                body = json.dumps({"jsonrpc": "2.0", "id": 1, "method": "tools/list"}).encode()
                request = urllib.request.Request(url, data=body, headers={"Content-Type": "application/json"}, method="POST")
                with self.assertRaises(urllib.error.HTTPError) as failure:
                    urllib.request.urlopen(request, timeout=2)
                self.assertEqual(failure.exception.code, 401)
                request.add_header("Authorization", "Bearer test-only-token")
                with urllib.request.urlopen(request, timeout=2) as response:
                    value = json.loads(response.read().decode("utf-8"))
                self.assertEqual(response.status, 200)
                self.assertEqual({tool["name"] for tool in value["result"]["tools"]}, {"formula_hint", "unit_convert"})
            finally:
                httpd.shutdown()
                httpd.server_close()
                thread.join(timeout=2)


if __name__ == "__main__":
    unittest.main()
