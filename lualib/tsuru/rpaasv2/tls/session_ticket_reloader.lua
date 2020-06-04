-- Copyright 2020 tsuru authors. All rights reserved.
-- Use of this source code is governed by a BSD-style
-- license that can be found in the LICENSE file.
--

local _M = {}
local _m = {}

local inotify        = require('inotify')
local ngx            = require('ngx')

local rpaasv2_session_ticket = require('tsuru.rpaasv2.tls.session_ticket')

local options

local unix_path_pattern = [[^(.+)/(.+)$]]

local function dir_name(path)
    return path:gsub(unix_path_pattern, '%1')
end

local function base_name(path)
    return path:gsub(unix_path_pattern, '%2')
end

local function read_file(filename)
    local file = io.open(filename, 'rb')
    if not file then
      return '', 'failed to open the file ' .. filename
    end

    local content = file:read('*a')
    file:close()

    return content, nil
end

local function session_ticket_reloader(premature, self)
    if not self.handle then
        ngx.log(ngx.ERR, "inotify handle not provided")
        return
    end

    if premature then
        ngx.log(ngx.DEBUG, 'cleaning up the session ticket reloader')
        self.handle:close()
        return
    end

    for event in self.handle:events() do
        if event.name == self.ticket_base_name or event.name == '..data' then
            self:update_current_encryption_key()
        end
    end
end

function _M:new(opts)
    if type(opts) ~= 'table' then
        return nil, 'opts must be a table'
    end

    local m = setmetatable({}, { __index = _m })

    m.ticket_file      = opts.ticket_file
    if type(m.ticket_file) ~= 'string' or not m.ticket_file then
        return nil, 'session ticket encryption key file (ticket_file) cannot be either non-string or empty'
    end

    m.ticket_dir_name  = dir_name(m.ticket_file)
    m.ticket_base_name = base_name(m.ticket_file)

    m.retain_last_keys = opts.retain_last_keys or 1
    if type(m.retain_last_keys) ~= 'number' or m.retain_last_keys < 1 then
        return nil, 'number of keys (retain_last_keys) must be an integer number (greater than zero)'
    end

    m.sync_interval = opts.sync_interval or 5
    if type(m.sync_interval) ~= 'number' or m.sync_interval < 1 then
        return nil, 'sync interval (sync_interval) must be a integer number (greater than zero seconds)'
    end

    return m, nil
end

function _m:start_worker()
    local worker_id = ngx.worker.id()
    if worker_id ~= 0 then -- not the first nginx worker
        ngx.log(ngx.DEBUG, 'skipping execution of this worker: ', worker_id)
        return
    end

    ngx.log(ngx.NOTICE, 'Running TLS session ticket encryption key reloader')
    ngx.log(ngx.NOTICE, 'Watching for changes of ', self.ticket_base_name,' file in ', self.ticket_dir_name, ' directory every ', self.sync_interval, 's')

    self.handle = inotify.init({ blocking = false })
    self.handle:addwatch(self.ticket_dir_name, bit.bor(inotify.IN_CREATE, inotify.IN_MODIFY, inotify.IN_MOVED_TO))

    local ok, err = ngx.timer.every(self.sync_interval, session_ticket_reloader, self)
    if not ok then
        ngx.log(ngx.ERR, 'failed to create timer: ', err)
    end
end

function _m:update_current_encryption_key()
    local key, err = read_file(self.ticket_file)
    if err then
        ngx.log(ngx.ERR, 'failed to read the current token: ', err)
        return false, err
    end

    local new_key_digest = ngx.md5(key)
    ngx.log(ngx.DEBUG, 'New key MD5 digest: ', new_key_digest)
    ngx.log(ngx.DEBUG, 'Current key MD5 digest: ', self.current_key_digest)

    if self.current_key_digest == new_key_digest then
        ngx.log(ngx.NOTICE, 'nothing to update due to the new key equals to the current one')
        return false, nil
    end

    local err = rpaasv2_session_ticket.update_ticket_encryption_key(key, self.retain_last_keys)
    if err then
        ngx.log(ngx.ERR, 'failed to update the new encryption key: ', err)
        return false, err
    end

    ngx.log(ngx.NOTICE, 'New encryption key (', new_key_digest ,') updated')
    self.current_key_digest = new_key_digest

    return true, nil
end

return _M
