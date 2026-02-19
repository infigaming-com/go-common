package snowflake

import _ "embed"

// claimNodeLua atomically claims the first available node ID (0-1023).
// KEYS: none (uses prefix + iteration)
// ARGV[1]: key prefix (e.g. "snowflake:node:")
// ARGV[2]: holder identity string
// ARGV[3]: TTL in seconds
// Returns: claimed node ID, or -1 if all slots occupied.
const claimNodeLua = `
local prefix = ARGV[1]
local holder = ARGV[2]
local ttl = tonumber(ARGV[3])
for i = 0, 1023 do
    local key = prefix .. i
    local ok = redis.call("SET", key, holder, "NX", "EX", ttl)
    if ok then
        return i
    end
end
return -1
`

// renewLeaseLua atomically renews a lease if held by the same holder.
// KEYS[1]: lease key
// ARGV[1]: expected holder
// ARGV[2]: TTL in seconds
// Returns: 1 if renewed, 0 if not held by this holder.
const renewLeaseLua = `
local current = redis.call("GET", KEYS[1])
if current == ARGV[1] then
    redis.call("EXPIRE", KEYS[1], tonumber(ARGV[2]))
    return 1
end
return 0
`

// releaseLeaseLua atomically releases a lease if held by the same holder.
// KEYS[1]: lease key
// ARGV[1]: expected holder
// Returns: 1 if released, 0 if not held by this holder.
const releaseLeaseLua = `
local current = redis.call("GET", KEYS[1])
if current == ARGV[1] then
    redis.call("DEL", KEYS[1])
    return 1
end
return 0
`
