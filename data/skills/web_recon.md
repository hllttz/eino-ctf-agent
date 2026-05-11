---
name: web_recon
title: Web 信息收集
description: 对目标网站进行信息收集，包括端口扫描、子域名枚举、目录爆破。
priority: 10
enabled: true
triggers:
  - web
  - 信息收集
  - 扫描
  - 子域名
max_tokens: 2000
---

## 步骤

1. 使用 nmap 扫描目标端口
2. 使用 subfinder 枚举子域名
3. 使用 dirsearch 爆破目录
4. 整理结果并输出报告

## 注意事项

- 只在授权目标上执行
- 控制扫描速率避免触发 WAF
