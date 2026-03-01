# Dense RAG MCP Server Demo Script
# This script demonstrates how to interact with the MCP server

Write-Host "Dense RAG MCP Server Demo" -ForegroundColor Green
Write-Host "=========================" -ForegroundColor Green
Write-Host ""

# Check if the MCP server executable exists
if (-not (Test-Path "bin\dense-rag-mcp.exe")) {
    Write-Host "Error: dense-rag-mcp.exe not found!" -ForegroundColor Red
    Write-Host "Please build it first with: go build -o bin\dense-rag-mcp.exe .\cmd\dense-rag-mcp" -ForegroundColor Yellow
    exit 1
}

Write-Host "Starting MCP server..." -ForegroundColor Yellow

# Start the MCP server process
$process = Start-Process -FilePath "bin\dense-rag-mcp.exe" -PassThru -NoNewWindow -RedirectStandardInput -RedirectStandardOutput -RedirectStandardError

Start-Sleep -Seconds 2

if ($process.HasExited) {
    Write-Host "Error: MCP server failed to start" -ForegroundColor Red
    Write-Host "Error output:" -ForegroundColor Red
    Get-Content $process.StandardError.ReadToEnd()
    exit 1
}

Write-Host "MCP server started successfully!" -ForegroundColor Green
Write-Host ""
Write-Host "The MCP server is now running and ready to accept JSON-RPC requests via stdin/stdout." -ForegroundColor Cyan
Write-Host ""
Write-Host "Example requests you can send:" -ForegroundColor Yellow
Write-Host "1. Initialize:" -ForegroundColor White
Write-Host '   {"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}' -ForegroundColor Gray
Write-Host ""
Write-Host "2. List tools:" -ForegroundColor White
Write-Host '   {"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' -ForegroundColor Gray
Write-Host ""
Write-Host "3. Get stats:" -ForegroundColor White
Write-Host '   {"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_stats","arguments":{}}}' -ForegroundColor Gray
Write-Host ""
Write-Host "4. Semantic search:" -ForegroundColor White
Write-Host '   {"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"semantic_search","arguments":{"query":"your search query","top_k":5}}}' -ForegroundColor Gray
Write-Host ""
Write-Host "Press Ctrl+C to stop the server..." -ForegroundColor Yellow

# Wait for user to stop the process
try {
    $process.WaitForExit()
} catch {
    Write-Host "Stopping MCP server..." -ForegroundColor Yellow
    $process.Kill()
}

Write-Host "MCP server stopped." -ForegroundColor Green