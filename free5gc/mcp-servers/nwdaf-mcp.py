#!/usr/bin/env python3
"""
NWDAF MCP Server — exposes free5gc NWDAF analytics as MCP tools.

Tools:
  get_nf_load        — NF load/CPU/mem per NF (AMF, SMF, UPF)
  get_ue_mobility    — UE registration, connectivity, location changes
  get_ue_communication — PDU session analytics per UE
  get_anomalies      — Abnormal behaviour detection (re-reg, flapping)
  get_recommendations — Aggregated recommendations from prediction models
  get_all_analytics  — Fetch all 5 endpoints in one call
"""

import asyncio
import json
from typing import Any

import httpx
from mcp.server.fastmcp import FastMCP

NWDAF_BASE = "http://127.0.0.47:8000/nnwdaf-analyticsinfo/v1/analytics"
NWDAF_MGMT_BASE = "http://127.0.0.47:8000/nnwdaf-management/v1/nf-subscriptions"
TIMEOUT = 5.0

mcp = FastMCP(
    "nwdaf",
    instructions=(
        "NWDAF analytics for the free5gc 5G core. "
        "Query NF load, UE mobility, UE communication, anomalies, and recommendations. "
        "The 5G core must be running for these tools to return data."
    ),
)


async def _query(event_id: str) -> dict[str, Any]:
    """Query a single NWDAF analytics endpoint."""
    async with httpx.AsyncClient(timeout=TIMEOUT) as client:
        resp = await client.get(NWDAF_BASE, params={"event-id": event_id})
        resp.raise_for_status()
        return resp.json()


def _fmt(data: dict) -> str:
    """Pretty-print JSON for tool output."""
    return json.dumps(data, indent=2)


@mcp.tool()
async def get_nf_load() -> str:
    """Get NF load analytics (CPU, memory, load percentage) for AMF, SMF, UPF.
    Returns per-NF metrics including event rates and active UE counts from collected data."""
    try:
        data = await _query("NF_LOAD")
        return _fmt(data)
    except Exception as e:
        return f"Error querying NF_LOAD: {e}"


@mcp.tool()
async def get_ue_mobility(supi: str = "") -> str:
    """Get UE mobility analytics — registration state, connectivity, location changes.
    Returns per-UE trajectories with isRegistered/isConnected status.
    Optional: pass a SUPI (e.g. 'imsi-208930000000001') to filter results."""
    try:
        data = await _query("UE_MOBILITY")
        if supi and data.get("ueMobilityAnalytics"):
            analytics = data["ueMobilityAnalytics"]
            trajectories = analytics.get("ueTrajectories", [])
            filtered = [t for t in trajectories if t.get("supi") == supi]
            if filtered:
                analytics["ueTrajectories"] = filtered
                analytics["uniqueUes"] = len(filtered)
        return _fmt(data)
    except Exception as e:
        return f"Error querying UE_MOBILITY: {e}"


@mcp.tool()
async def get_ue_communication(supi: str = "") -> str:
    """Get UE communication analytics — PDU session establishment/release rates, active sessions.
    Returns per-UE session stats and aggregate communication metrics.
    Optional: pass a SUPI to filter results."""
    try:
        data = await _query("UE_COMMUNICATION")
        if supi and data.get("ueCommunicationAnalytics"):
            analytics = data["ueCommunicationAnalytics"]
            stats = analytics.get("sessionStats", [])
            filtered = [s for s in stats if s.get("supi") == supi]
            if filtered:
                analytics["sessionStats"] = filtered
        return _fmt(data)
    except Exception as e:
        return f"Error querying UE_COMMUNICATION: {e}"


@mcp.tool()
async def get_anomalies() -> str:
    """Get abnormal behaviour analytics — detects RAPID_REREGISTRATION, SESSION_FLAPPING,
    CONNECTIVITY_FLAPPING within a 5-minute sliding window. Returns anomaly count, per-UE
    reports with severity scores (LOW/MEDIUM/HIGH)."""
    try:
        data = await _query("ABNORMAL_BEHAVIOUR")
        return _fmt(data)
    except Exception as e:
        return f"Error querying ABNORMAL_BEHAVIOUR: {e}"


@mcp.tool()
async def get_recommendations() -> str:
    """Get NWDAF recommendations — aggregated from mobility prediction, session load
    forecasting, and anomaly scoring models. Returns prioritized actionable recommendations
    (e.g. scale UPF, investigate anomalous UEs, enable mobility-optimized bearers)."""
    try:
        data = await _query("NWDAF_RECOMMENDATION")
        return _fmt(data)
    except Exception as e:
        return f"Error querying NWDAF_RECOMMENDATION: {e}"


@mcp.tool()
async def get_all_analytics() -> str:
    """Fetch all 5 NWDAF analytics endpoints in parallel and return a combined result.
    Use this for a complete network snapshot instead of calling each tool individually."""
    event_ids = [
        "NF_LOAD",
        "UE_MOBILITY",
        "UE_COMMUNICATION",
        "ABNORMAL_BEHAVIOUR",
        "NWDAF_RECOMMENDATION",
    ]
    results = {}
    async with httpx.AsyncClient(timeout=TIMEOUT) as client:
        tasks = []
        for eid in event_ids:
            tasks.append(client.get(NWDAF_BASE, params={"event-id": eid}))
        responses = await asyncio.gather(*tasks, return_exceptions=True)
        for eid, resp in zip(event_ids, responses):
            if isinstance(resp, Exception):
                results[eid] = {"error": str(resp)}
            else:
                try:
                    results[eid] = resp.json()
                except Exception:
                    results[eid] = {"error": f"HTTP {resp.status_code}: {resp.text[:200]}"}
    return _fmt(results)


@mcp.tool()
async def get_subscription_status() -> str:
    """Get the current outbound event subscription status for AMF and SMF.
    Shows whether NWDAF is subscribed, the subscription ID, NF URI, and active event list."""
    try:
        async with httpx.AsyncClient(timeout=TIMEOUT) as client:
            resp = await client.get(NWDAF_MGMT_BASE)
            resp.raise_for_status()
            return _fmt(resp.json())
    except Exception as e:
        return f"Error getting subscription status: {e}"


@mcp.tool()
async def manage_nf_subscription(nf: str, action: str, events: list[str] = []) -> str:
    """Subscribe or unsubscribe NWDAF from an NF's event stream at runtime.

    Args:
        nf:     Target NF — 'amf' or 'smf'
        action: 'subscribe' or 'unsubscribe'
        events: Optional list of event names to subscribe to (e.g. ['LOCATION_REPORT',
                'REGISTRATION_STATE_REPORT']). Only used for subscribe. If omitted,
                the default event list from nwdafcfg.yaml is used.

    Returns confirmation or error detail.

    Examples:
        Subscribe to AMF with default events:
            manage_nf_subscription(nf='amf', action='subscribe')
        Subscribe to SMF with specific events only:
            manage_nf_subscription(nf='smf', action='subscribe', events=['PDU_SES_EST','PDU_SES_REL'])
        Unsubscribe from AMF:
            manage_nf_subscription(nf='amf', action='unsubscribe')
    """
    nf = nf.lower().strip()
    if nf not in ("amf", "smf"):
        return f"Error: nf must be 'amf' or 'smf', got '{nf}'"
    if action not in ("subscribe", "unsubscribe"):
        return f"Error: action must be 'subscribe' or 'unsubscribe', got '{action}'"

    url = f"{NWDAF_MGMT_BASE}/{nf}"

    try:
        async with httpx.AsyncClient(timeout=TIMEOUT) as client:
            if action == "subscribe":
                body = {"events": events} if events else {}
                resp = await client.post(url, json=body)
            else:
                resp = await client.delete(url)

            if resp.status_code == 204:
                return f"Successfully unsubscribed from {nf.upper()} events."
            resp.raise_for_status()
            return _fmt(resp.json())
    except httpx.HTTPStatusError as e:
        try:
            detail = e.response.json()
            return f"Error ({e.response.status_code}): {_fmt(detail)}"
        except Exception:
            return f"Error ({e.response.status_code}): {e.response.text}"
    except Exception as e:
        return f"Error managing {nf.upper()} subscription: {e}"


if __name__ == "__main__":
    mcp.run()
