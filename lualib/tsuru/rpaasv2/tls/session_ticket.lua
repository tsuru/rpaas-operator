-- Copyright 2020 tsuru authors. All rights reserved.
-- Use of this source code is governed by a BSD-style
-- license that can be found in the LICENSE file.
--

local _M = {}

local ffi       = require('ffi')
local C         = ffi.C
local ffi_str   = ffi.string

local base      = require('resty.core.base')
local table     = require('table')

local get_string_buf    = base.get_string_buf
local get_errmsg_ptr    = base.get_errmsg_ptr
local void_ptr_type     = ffi.typeof('void*')
local void_ptr_ptr_type = ffi.typeof('void**')
local ptr_size          = ffi.sizeof(void_ptr_type)

ffi.cdef[[
int ngx_http_lua_ffi_get_ssl_ctx_count(void);

int ngx_http_lua_ffi_get_ssl_ctx_list(void **buf);

int ngx_http_lua_ffi_update_ticket_encryption_key(void *ctx, const unsigned char *key, unsigned int nkeys, const unsigned int key_length, char **err);
]]

local function get_ssl_contexts()
    local n = C.ngx_http_lua_ffi_get_ssl_ctx_count()
    if n < 0 then
        return nil, 'ssl context cannot be negative'
    end

    if n == 0 then
        return nil, nil
    end

    local buffer = ffi.cast(void_ptr_ptr_type, get_string_buf(ptr_size * n))
    local rc = ffi.C.ngx_http_lua_ffi_get_ssl_ctx_list(buffer)
    if rc ~= 0 then -- not NGX_OK
        return nil, 'cannot get the ssl contexts'
    end

    local ctxs = table.new(n, 0)
    for i = 1, n do
        ctxs[i] = buffer[i - 1]
    end

    return ctxs, nil
end

function _M.update_ticket_encryption_key(key, nkeys)
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
        local rc = C.ngx_http_lua_ffi_update_ticket_encryption_key(ctx, key, nkeys, #key, errmsg)
        if rc ~= 0 then -- not NGX_OK
            return 'failed to update the key into OpenSSL context: ' .. ffi_str(errmsg[0])
        end
    end

    return nil
end

return _M
