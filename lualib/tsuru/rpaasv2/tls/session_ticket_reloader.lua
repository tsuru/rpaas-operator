local _M = {}

--
-- Session ticket reloader package
--

local ffi       = require('ffi')
local C         = ffi.C
local ffi_str   = ffi.string

local base      = require "resty.core.base"
local table     = require "table"

local get_string_buf    = base.get_string_buf
local get_errmsg_ptr    = base.get_errmsg_ptr
local void_ptr_type     = ffi.typeof("void*")
local void_ptr_ptr_type = ffi.typeof("void**")
local ptr_size          = ffi.sizeof(void_ptr_type)

ffi.cdef[[
int ngx_http_lua_ffi_get_ssl_ctx_count(void);

int ngx_http_lua_ffi_get_ssl_ctx_list(void **buf);

int ngx_http_lua_ffi_update_ticket_encryption_key(void *ctx, const unsigned char *key, unsigned int nkeys, const unsigned int key_length, char **err);
]]

local function get_ssl_contexts()
    ngx.log(ngx.DEBUG, 'running get_ssl_contexts() function')

    local n = C.ngx_http_lua_ffi_get_ssl_ctx_count()
    if n < 0 then
        ngx.log(ngx.ERR, 'ssl contexts cannot be negative (got ', n, ')')
        return nil, 'ssl context cannot be negative'
    end

    if n == 0 then
        ngx.log(ngx.DEBUG, 'no ssl contexts')
        return nil, nil
    end

    ngx.log(ngx.DEBUG, 'expected number of ssl contexts: ', n)

    local buffer = ffi.cast(void_ptr_ptr_type, get_string_buf(ptr_size * n))
    local rc = ffi.C.ngx_http_lua_ffi_get_ssl_ctx_list(buffer)
    if rc ~= 0 then -- not NGX_OK
        ngx.log(ngx.ERR, 'failed to retrieve the ssl contexts: code ', rc)
        return nil, 'cannot get the ssl contexts'
    end

    local ctxs = table.new(n, 0)
    for i = 1, n do
        ctxs[i] = buffer[i - 1]
    end

    ngx.log(ngx.DEBUG, 'get ssl contexts succeeded')
    return ctxs, nil
end

local function update_ticket_encryption_key(key, retain_last_keys)
    ngx.log(ngx.DEBUG, 'running update_ticket_encryption_key() function')

    local ssl_contexts, err = get_ssl_contexts()
    if err then
        return err
    end

    if not ssl_contexts or #ssl_contexts == 0 then
        return 'no ssl ctx set'
    end

    if #key ~= 48 and #key ~= 80 then
        return 'ssl ticket key must either have 48 or 80 bytes'
    end

    for _, ctx in ipairs(ssl_contexts) do
        local errmsg = get_errmsg_ptr()
        local rc = C.ngx_http_lua_ffi_update_ticket_encryption_key(ctx, key, retain_last_keys, #key, errmsg)
        if rc ~= 0 then -- not NGX_OK
            return 'failed to update the key into OpenSSL context: ' .. ffi_str(errmsg[0])
        end
    end

    return nil
end

-- My module begins here
--
--

local inotify = require('inotify')
local ngx     = require('ngx')

local options

local unix_path_pattern = [[^(.+)/(.+)$]]

local function dir_name(path)
  return path:gsub(unix_path_pattern, '%1')
end

local function base_name(path)
  return path:gsub(unix_path_pattern, '%2')
end

local function read_file(filename)
  ngx.log(ngx.DEBUG, 'reading the ', filename, ' file')

  local file = io.open(filename, 'rb')
  if not file then
    return '', 'failed to open the file ' .. filename
  end

  local content = file:read('*a')
  file:close()

  return content, nil
end

local function update_stek()
  local ticket, err = read_file(options.ticket_file)
  if err then
    ngx.log(ngx.ERR, "failed to read the current token: ", err)
  end

  ngx.log(ngx.DEBUG, "ticket: ", ngx.md5(ticket))

  local err = update_ticket_encryption_key(ticket, options.retain_last_keys)
  if err then
    ngx.log(ngx.ERR, "failed to update the new ticket: ", err)
  end
end

local function session_ticket_reloader(premature, handler)
  ngx.log(ngx.DEBUG, 'running session_ticket_reloader() function')

  if not handler then
    ngx.log(ngx.ERR, "inotify handler not provided")
    return
  end

  if premature then
    ngx.log(ngx.DEBUG, 'cleaning up the session ticket reloader')
    handler:close()
    return
  end

  for event in handler:events() do
    ngx.log(ngx.DEBUG, 'changes on ', event.name, ' detected')

    if event.name == options.ticket_base_name or event.name == '..data' then
      update_stek()
    end
  end

  ngx.log(ngx.DEBUG, 'finishing session_ticket_reloader() function')
end

local function init(opts)
  opts = opts or {}

  options = {
    ticket_file      = opts.ticket_file      or '/etc/nginx/ticket.key',
    retain_last_keys = opts.retain_last_keys or 2,
    sync_interval    = opts.sync_interval    or 5,
  }

  options.ticket_dir_name = dir_name(options.ticket_file)
  options.ticket_base_name = base_name(options.ticket_file)

  if options.retain_last_keys < 1 then
    return 'retain last keys must be greater than one'
  end

  if options.sync_interval < 1 then
    return 'sync interval must be greater than zero seconds'
  end

  return nil
end

function _M:start_worker(opts)
  local worker_id = ngx.worker.id()
  if worker_id ~= 0 then -- not the first nginx worker
    ngx.log(ngx.DEBUG, 'skipping execution of this worker: ', worker_id)
    return
  end

  ngx.log(ngx.DEBUG, 'running session_ticket_reloader worker')

  ngx.log(ngx.DEBUG, 'Ticket file: ', opts.ticket_file)

  assert(not init(opts), 'failed to initilize module configuration')

  ngx.log(ngx.NOTICE, 'watching for changes on ', options.ticket_dir_name, ' directory')

  local handler = inotify.init({ blocking = false })
  local inotify_event_mask = bit.bor(inotify.IN_CREATE, inotify.IN_MODIFY, inotify.IN_MOVED_TO)
  handler:addwatch(options.ticket_dir_name, inotify_event_mask)

  local ok, err = ngx.timer.every(options.sync_interval, session_ticket_reloader, handler)
  if not ok then
    ngx.log(ngx.ERR, 'failed to create timer: ', err)
    return
  end
end

return _M
