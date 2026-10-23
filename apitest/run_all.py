from __future__ import annotations

import argparse
import html
import importlib
import sys
import time
import traceback
from datetime import datetime
from pathlib import Path
from urllib.parse import urlsplit, urlunsplit

from common import Context, read_dotenv

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(Path(__file__).resolve().parent))

TESTS = [
    "api.login.post",
    "api.me.get",
    "api.knowledge.search.post",
    "api.prompts.get",
    "api.prompts.post",
    "api.prompts.id.put",
    "api.skills.get",
    "api.mcp.servers.get",
    "api.mcp.servers.post",
    "api.mcp.servers.id.put",
    "api.mcp.servers.id.connect.post",
    "api.mcp.servers.id.tools.get",
    "api.skills.post",
    "api.skills.id.put",
    "api.sessions.post",
    "api.sessions.get",
    "api.sessions.id.messages.post",
    "api.sessions.id.messages.get",
    "api.mcp.servers.id.disconnect.post",
    "api.mcp.tool_calls.get",
    "api.mcp.servers.id.delete",
    "api.materials.id.delete",
]


def main():
    parser = argparse.ArgumentParser(description="Run CampusClaw API acceptance tests and write an HTML report.")
    parser.add_argument("--base-url", default=read_dotenv(ROOT / ".env").get("APITEST_BASE_URL", "http://localhost:8080"))
    args = parser.parse_args()
    context = Context(args.base_url.rstrip("/"))
    results = []
    suite_start = time.perf_counter()
    for name in TESTS:
        started = time.perf_counter()
        try:
            result = importlib.import_module(name).run(context)
            results.append((name, "PASS", str(result or "验证通过"), time.perf_counter() - started))
        except Exception as error:
            detail = "".join(traceback.format_exception_only(type(error), error)).strip()
            results.append((name, "FAIL", detail, time.perf_counter() - started))
    cleanup_errors = context.cleanup_created_data()
    if cleanup_errors:
        results.append(("test-data-cleanup", "FAIL", "; ".join(cleanup_errors), 0.0))
    passed = sum(1 for _, state, _, _ in results if state == "PASS")
    failed = len(results) - passed
    duration = time.perf_counter() - suite_start
    report_dir = Path(__file__).resolve().parent / "reports"
    report_dir.mkdir(parents=True, exist_ok=True)
    report_path = report_dir / f"api-test-{context.run_id}.html"
    parsed = urlsplit(context.base_url)
    safe_host = parsed.hostname or ""
    if parsed.port:
        safe_host += f":{parsed.port}"
    safe_target = urlunsplit((parsed.scheme, safe_host, parsed.path, "", ""))
    rows = "\n".join(
        f"<tr class='{state.lower()}'><td>{html.escape(name.replace('.', '/'))}</td><td>{state}</td><td>{html.escape(detail)}</td><td>{elapsed:.2f}s</td></tr>"
        for name, state, detail, elapsed in results
    )
    report_path.write_text(f"""<!doctype html><html lang='zh-CN'><meta charset='utf-8'><title>CampusClaw API 测试报告</title>
<style>body{{font:15px/1.6 system-ui,'Microsoft YaHei',sans-serif;margin:36px;color:#263244}}h1{{color:#a93238}}.summary{{padding:14px;background:#f4f6f8;border-radius:9px}}table{{width:100%;border-collapse:collapse;margin-top:20px}}th,td{{padding:10px;border-bottom:1px solid #e6e8eb;text-align:left;vertical-align:top}}.pass td:nth-child(2){{color:#167545;font-weight:700}}.fail td:nth-child(2){{color:#b42318;font-weight:700}}code{{overflow-wrap:anywhere}}</style>
<h1>CampusClaw API 测试报告</h1><p>目标服务：{html.escape(safe_target)}　·　生成时间：{datetime.now().astimezone().isoformat(timespec='seconds')}</p>
<div class='summary'>运行标识：{html.escape(context.run_id)}　　PASS：{passed}　　FAIL：{failed}　　新建测试数据清理：{'失败（详见下表）' if cleanup_errors else '成功'}　　耗时：{duration:.2f}s</div>
<table><thead><tr><th>接口脚本</th><th>结果</th><th>响应与一致性检查</th><th>耗时</th></tr></thead><tbody>{rows}</tbody></table></html>""", encoding="utf-8")
    print(f"API_TEST_SUMMARY PASS={passed} FAIL={failed}")
    print(f"HTML_REPORT={report_path}")
    return 0 if failed == 0 else 1


if __name__ == "__main__":
    raise SystemExit(main())
