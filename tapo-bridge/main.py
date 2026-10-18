import asyncio
import os
import time
from fastapi import FastAPI, HTTPException
from tapo import ApiClient

app = FastAPI()
client = ApiClient(os.environ["TAPO_EMAIL"], os.environ["TAPO_PASSWORD"])
_plugs: dict[str, object] = {}
_cache: dict[str, dict] = {}
_locks: dict[str, asyncio.Lock] = {}
CACHE_SECONDS = 3


def lock_for(ip: str) -> asyncio.Lock:
    if ip not in _locks:
        _locks[ip] = asyncio.Lock()
    return _locks[ip]


async def plug(ip: str):
    if ip not in _plugs:
        _plugs[ip] = await client.p110(ip)
    return _plugs[ip]


def forget(ip: str):
    _plugs.pop(ip, None)


async def read(ip: str) -> dict:
    async with lock_for(ip):
        c = _cache.get(ip)
        if c and time.time() - c["ts"] < CACHE_SECONDS:
            return c
        p = await plug(ip)
        info = await p.get_device_info()
        usage = await p.get_energy_usage()
        # P110 reports current_power in milliwatts; verify against your firmware.
        c = {"on": info.device_on, "power_w": usage.current_power / 1000, "ts": time.time()}
        _cache[ip] = c
        return c


@app.get("/{ip}/state")
async def state(ip: str):
    try:
        c = await read(ip)
        return {"on": c["on"], "power_w": c["power_w"]}
    except Exception as e:
        forget(ip)
        raise HTTPException(502, str(e))


@app.post("/{ip}/on")
async def turn_on(ip: str):
    async with lock_for(ip):
        try:
            await (await plug(ip)).on()
            _cache.pop(ip, None)
            return {"ok": True}
        except Exception as e:
            forget(ip)
            raise HTTPException(502, str(e))


@app.post("/{ip}/off")
async def turn_off(ip: str):
    async with lock_for(ip):
        try:
            await (await plug(ip)).off()
            _cache.pop(ip, None)
            return {"ok": True}
        except Exception as e:
            forget(ip)
            raise HTTPException(502, str(e))
