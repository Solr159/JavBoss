package client

import (
	"html/template"
	"net/http"
)

var setupPage = template.Must(template.New("client-setup").Parse(`<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <title>JavBoss Client</title>
  <style>
    :root{color-scheme:dark}*{box-sizing:border-box}body{margin:0;background:#09090b;color:#e4e4e7;font:15px/1.5 system-ui,sans-serif;min-height:100vh;display:grid;place-items:center}.card{width:min(560px,calc(100vw - 32px));padding:28px;border:1px solid #27272a;border-radius:16px;background:#18181b;box-shadow:0 20px 60px #0008}h1{margin:0 0 8px;font-size:24px}.hint{margin:0 0 22px;color:#a1a1aa}.error{padding:10px 12px;margin-bottom:16px;border-radius:8px;background:#7f1d1d;color:#fecaca}label{display:block;margin-bottom:7px;font-weight:600}input{width:100%;padding:11px 12px;border:1px solid #3f3f46;border-radius:9px;background:#09090b;color:#fafafa;font:inherit}button{width:100%;margin-top:16px;padding:11px;border:0;border-radius:9px;background:#e4e4e7;color:#18181b;font:inherit;font-weight:700;cursor:pointer}.current{margin-top:18px;color:#71717a;font-size:13px;word-break:break-all}
  </style>
</head>
<body><main class="card">
  <h1>JavBoss Client</h1>
  <p class="hint">输入远端 JavBoss Server 地址。本机 Client 将代理页面和数据，并使用本机 MPV 播放远程视频。</p>
  {{if .Error}}<div class="error">{{.Error}}</div>{{end}}
  <form method="post" action="/__client/setup">
    <label for="server_url">远端 Server 地址</label>
    <input id="server_url" name="server_url" type="url" required autofocus placeholder="https://javboss.example.com" value="{{.ServerURL}}">
    <button type="submit">保存并连接</button>
  </form>
  {{if .ServerURL}}<div class="current">当前地址：{{.ServerURL}}</div>{{end}}
</main></body></html>`))

type setupPageData struct {
	ServerURL string
	Error     string
}

func renderSetup(w http.ResponseWriter, status int, data setupPageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = setupPage.Execute(w, data)
}
