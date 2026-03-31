#!/usr/bin/env python3
"""
SBI Gateway MCP Server — OAuth-aware access to NRF NFM and AMF/SMF OAM APIs.

Handles OAuth2 token acquisition automatically:
1. Registers a lightweight NF with NRF via PUT (no auth needed)
2. Requests tokens per target service (NRF, AMF, SMF)
3. Caches tokens (1000s TTL)

Tools:
  NRF:
    list_nf_instances     — List all registered NF UUIDs
    get_nf_profile        — Get a specific NF's full profile
    check_nrf_integrity   — Audit all NFs for rogue registrations
  AMF:
    get_registered_ues    — List all UEs registered with AMF
    get_ue_context        — Get a specific UE's context by SUPI
  SMF:
    get_user_plane_info   — Get UPF associations and user plane status
"""

import json
import time
import uuid
from typing import Any

import httpx
from mcp.server.fastmcp import FastMCP

# --- Configuration ---
NRF_BASE = "http://127.0.0.10:8000"
AMF_BASE = "http://127.0.0.18:8000"
SMF_BASE = "http://127.0.0.2:8000"
TIMEOUT = 5.0

# Generate a proper RFC 4122 v4 UUID, same as Go NFs use via uuid.New()
MCP_AGENT_UUID = str(uuid.uuid4())
MCP_NF_TYPE = "NWDAF"  # Register as NWDAF to get access to OAM scopes

# Expected NF IPs for integrity checks
EXPECTED_NF_IPS = {
    "NRF": "127.0.0.10",
    "AMF": "127.0.0.18",
    "SMF": "127.0.0.2",
    "UPF": "127.0.0.8",
    "NWDAF": "127.0.0.47",
}

mcp = FastMCP(
    "sbi-gateway",
    instructions=(
        "OAuth-aware gateway to free5gc NRF, AMF, and SMF SBI APIs. "
        "Provides NF discovery, UE context, and user plane info. "
        "Handles OAuth2 token acquisition automatically."
    ),
)

# --- Token cache ---
_token_cache: dict[str, tuple[str, float]] = {}  # scope -> (token, expiry_time)
_registered = False


async def _register_with_nrf(client: httpx.AsyncClient) -> bool:
    """Register the MCP agent as a lightweight NF with NRF (no auth needed)."""
    global _registered
    if _registered:
        return True

    profile = {
        "nfInstanceId": MCP_AGENT_UUID,
        "nfType": MCP_NF_TYPE,
        "nfStatus": "REGISTERED",
        "ipv4Addresses": ["127.0.0.1"],
        "nfServices": [],
    }
    try:
        resp = await client.put(
            f"{NRF_BASE}/nnrf-nfm/v1/nf-instances/{MCP_AGENT_UUID}",
            json=profile,
        )
        if resp.status_code in (200, 201):
            _registered = True
            return True
        return False
    except Exception:
        return False


async def _get_token(client: httpx.AsyncClient, target_nf_type: str, scope: str) -> str | None:
    """Get an OAuth2 token for the given target service, using cache when possible."""
    global _registered
    now = time.time()

    # Check cache
    if scope in _token_cache:
        token, expiry = _token_cache[scope]
        if now < expiry - 30:  # 30s safety margin
            return token

    # Ensure we're registered
    if not await _register_with_nrf(client):
        return None

    # Request token
    try:
        resp = await client.post(
            f"{NRF_BASE}/oauth2/token",
            data={
                "grant_type": "client_credentials",
                "nfInstanceId": MCP_AGENT_UUID,
                "nfType": MCP_NF_TYPE,
                "targetNfType": target_nf_type,
                "scope": scope,
            },
            headers={"Content-Type": "application/x-www-form-urlencoded"},
        )
        if resp.status_code == 200:
            data = resp.json()
            token = data.get("access_token", "")
            expires_in = data.get("expires_in", 1000)
            _token_cache[scope] = (token, now + expires_in)
            return token
        if resp.status_code == 400:
            # NRF may have restarted — re-register and retry once
            _registered = False
            _token_cache.clear()
            if await _register_with_nrf(client):
                resp = await client.post(
                    f"{NRF_BASE}/oauth2/token",
                    data={
                        "grant_type": "client_credentials",
                        "nfInstanceId": MCP_AGENT_UUID,
                        "nfType": MCP_NF_TYPE,
                        "targetNfType": target_nf_type,
                        "scope": scope,
                    },
                    headers={"Content-Type": "application/x-www-form-urlencoded"},
                )
                if resp.status_code == 200:
                    data = resp.json()
                    token = data.get("access_token", "")
                    expires_in = data.get("expires_in", 1000)
                    _token_cache[scope] = (token, now + expires_in)
                    return token
        return None
    except Exception:
        return None


async def _authed_get(url: str, target_nf_type: str, scope: str) -> dict[str, Any]:
    """Make an authenticated GET request, handling token acquisition."""
    async with httpx.AsyncClient(timeout=TIMEOUT) as client:
        token = await _get_token(client, target_nf_type, scope)
        headers = {}
        if token:
            headers["Authorization"] = f"Bearer {token}"

        resp = await client.get(url, headers=headers)

        # If 401 and we had a token, clear cache and retry once
        if resp.status_code == 401 and token:
            _token_cache.pop(scope, None)
            token = await _get_token(client, target_nf_type, scope)
            if token:
                headers["Authorization"] = f"Bearer {token}"
                resp = await client.get(url, headers=headers)

        if resp.status_code == 204:
            return {"status": 204, "message": "No content"}
        resp.raise_for_status()
        return resp.json()


def _fmt(data: Any) -> str:
    return json.dumps(data, indent=2, default=str)


# ========== NRF NFM Tools ==========

# DISABLED: NRF has a bug where GET /nf-instances uses c.Params.ByName() instead
# of c.Query() for query params, so nf-type and limit are never read. Returns 400.
# Fix requires patching NFs/nrf/internal/sbi/api_nfmanagement.go lines 201-202.
#
# @mcp.tool()
# async def list_nf_instances(nf_type: str = "") -> str:
#     """List all NF instances registered with NRF.
#     Optional: filter by nf_type (e.g. 'AMF', 'SMF', 'UPF').
#     Returns a list of NF instance UUIDs."""
#     try:
#         url = f"{NRF_BASE}/nnrf-nfm/v1/nf-instances"
#         if nf_type:
#             url += f"?nf-type={nf_type.upper()}"
#         data = await _authed_get(url, "NRF", "nnrf-nfm")
#         return _fmt(data)
#     except Exception as e:
#         return f"Error listing NF instances: {e}"


@mcp.tool()
async def get_nf_profile(nf_instance_id: str) -> str:
    """Get the full NF profile for a specific NF instance UUID.
    Returns: nfType, nfStatus, ipv4Addresses, nfServices, etc."""
    try:
        url = f"{NRF_BASE}/nnrf-nfm/v1/nf-instances/{nf_instance_id}"
        data = await _authed_get(url, "NRF", "nnrf-nfm")
        return _fmt(data)
    except Exception as e:
        return f"Error getting NF profile: {e}"


# DISABLED: Depends on list_nf_instances which is broken due to NRF query param bug.
# See comment above list_nf_instances for details.
#
# @mcp.tool()
# async def check_nrf_integrity() -> str:
#     """Audit all registered NFs for integrity issues."""
#     ...


# ========== AMF OAM Tools ==========

@mcp.tool()
async def get_registered_ues() -> str:
    """Get all UEs currently registered with the AMF.
    Returns UE contexts including SUPI, registration state, and connectivity info."""
    try:
        url = f"{AMF_BASE}/namf-oam/v1/registered-ue-context"
        data = await _authed_get(url, "AMF", "namf-oam")
        return _fmt(data)
    except Exception as e:
        return f"Error getting registered UEs: {e}"


@mcp.tool()
async def get_ue_context(supi: str) -> str:
    """Get detailed context for a specific UE by SUPI (e.g. 'imsi-208930000000001').
    Returns registration state, connectivity, PDU sessions, security context."""
    try:
        url = f"{AMF_BASE}/namf-oam/v1/registered-ue-context/{supi}"
        data = await _authed_get(url, "AMF", "namf-oam")
        return _fmt(data)
    except Exception as e:
        return f"Error getting UE context for {supi}: {e}"


# ========== SMF OAM Tools ==========

@mcp.tool()
async def get_user_plane_info() -> str:
    """Get user plane info from SMF — UPF associations, PFCP status, session counts."""
    try:
        url = f"{SMF_BASE}/nsmf-oam/v1/user-plane-info/"
        data = await _authed_get(url, "SMF", "nsmf-oam")
        return _fmt(data)
    except Exception as e:
        return f"Error getting user plane info: {e}"


@mcp.tool()
async def get_pdu_session_info(ref: str) -> str:
    """Get PDU session info for a specific session reference.
    The ref is typically obtained from UE context data."""
    try:
        url = f"{SMF_BASE}/nsmf-oam/v1/ue-pdu-session-info/{ref}"
        data = await _authed_get(url, "SMF", "nsmf-oam")
        return _fmt(data)
    except Exception as e:
        return f"Error getting PDU session info: {e}"


# ========== Combined Tools ==========

@mcp.tool()
async def get_network_overview() -> str:
    """Get a combined overview: registered UEs + user plane info.
    Single call that queries AMF and SMF in parallel.
    Note: NF instance listing is disabled due to NRF query param bug."""
    import asyncio

    async def _safe_query(name: str, coro):
        try:
            return name, await coro
        except Exception as e:
            return name, {"error": str(e)}

    tasks = [
        _safe_query("registered_ues", _authed_get(
            f"{AMF_BASE}/namf-oam/v1/registered-ue-context", "AMF", "namf-oam")),
        _safe_query("user_plane", _authed_get(
            f"{SMF_BASE}/nsmf-oam/v1/user-plane-info/", "SMF", "nsmf-oam")),
    ]
    results = await asyncio.gather(*tasks)
    overview = dict(results)
    return _fmt(overview)


if __name__ == "__main__":
    mcp.run()
