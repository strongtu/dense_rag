@echo off
echo Testing Dense RAG MCP Server...
echo.

echo Sending initialize request...
echo {"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}} | bin\dense-rag-mcp.exe

echo.
echo Test completed. Check the output above for any errors.
pause