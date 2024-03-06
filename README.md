<p align="center">
  <a href="https://github.com/gsh-lan/unwindia" target="blank"><img src="https://raw.githubusercontent.com/GSH-LAN/Unwindia/main/.resources/images/logo.png" height="128" alt="unwindia logo">
  <a href="https://github.com/gsh-lan/unwindia" target="blank"><img src="https://raw.githubusercontent.com/GSH-LAN/Unwindia/main/.resources/images/header.svg" height="128" alt="unwindia header" /></a>
</p>

# Unwindia_pterodactyl
> [Unwindia](https://github.com/GSH-LAN/Unwindia)'s integration with pterodactyl
---

This service provides an API which saves jobs into it's internal database and creates servers in pterodactyl according to API payload.

# THIS SERVICE IS A MESS AND JUST FULFILLS FIRST REQUIREMENTS - THIS SHOULD BE REWRITTEN

# Missing Features
* obtain steam gameserver token through [gameserver-token-api](https://github.com/gsh-lan/steam-gameserver-token-api)
* add gameserver "slug" to server name and add check for gameserver config matching and deletion & auto-recreation of server
* auto install server when not enough standby servers are available
* notifications
  * not enough standby servers are available
  * no more servers can be installed
  * server installation failed
  * 