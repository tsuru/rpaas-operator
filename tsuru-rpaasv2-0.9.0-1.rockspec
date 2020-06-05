package = "tsuru-rpaasv2"
version = "0.9.0-1"
source = {
   url = "git://github.com/tsuru/rpaas-operator.git",
   tag = "v0.9.0"
}
description = {
   summary = "Lua helpers to extend Nginx (OpenResty or just ngx_lua) features on RPaaS v2.",
   homepage = "https://github.com/tsuru/rpaas-operator",
   license = "3-clause BSD",
   maintainer = "Tsuru <tsuru@g.globo>"
}
dependencies = {
   "lua >= 5.1",
   "inotify ~> 0.5",
}
build = {
   type = "builtin",
   modules = {
      ["tsuru.rpaasv2.tls.session_ticket"] = "lualib/tsuru/rpaasv2/tls/session_ticket.lua",
      ["tsuru.rpaasv2.tls.session_ticket_reloader"] = "lualib/tsuru/rpaasv2/tls/session_ticket_reloader.lua"
   }
}
