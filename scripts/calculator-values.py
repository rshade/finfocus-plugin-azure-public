#!/usr/bin/env python3
"""Fill calculator-values.csv from the Azure Pricing Calculator backend.

Reads the JSON the calculator page itself loads from
https://azure.microsoft.com/api/{v2,v3}/pricing/<service>/calculator/ and never
the Retail Prices API (prices.azure.com), which is the plugin's own source.
Each row's monthly figure is computed the way the calculator computes it
(730 hours per month, graduated bands, free grants). A row the calculator
data cannot express unambiguously is left empty and the reason is printed.

Usage:
    scripts/calculator-values.py            # print values, change nothing
    scripts/calculator-values.py --write    # also write the CSV
"""

import argparse
import csv
import datetime
import json
import pathlib
import sys
import time
import urllib.error
import urllib.request

ROOT = pathlib.Path(__file__).resolve().parent.parent
CSV_PATH = ROOT / "internal" / "pricing" / "testdata" / "oracle" / "calculator-values.csv"
HOURS = 730
BASE = "https://azure.microsoft.com/api/{version}/pricing/{slug}/calculator/?culture=en-us&discount=mosp"
SERVICES = {
    "virtual-machines": "v3",
    "managed-disks": "v2",
    "storage": "v3",
    "app-service": "v2",
    "kubernetes-service": "v3",
    "azure-sql-database": "v3",
    "cosmos-db": "v3",
    "functions": "v2",
}


class Unfillable(Exception):
    """The calculator data cannot express this row without a guess."""


def fetch(slug, cache):
    if slug in cache:
        return cache[slug]
    url = BASE.format(version=SERVICES[slug], slug=slug)
    for attempt in range(4):
        req = urllib.request.Request(
            url,
            headers={"Accept": "application/json", "User-Agent": "finfocus-calculator-values"},
        )
        try:
            with urllib.request.urlopen(req, timeout=60) as resp:
                cache[slug] = (url, json.load(resp))
                return cache[slug]
        except (urllib.error.URLError, TimeoutError) as err:
            if attempt == 3:
                raise SystemExit(f"calculator backend unreachable: {url}: {err}")
            time.sleep(2 ** attempt)
    raise AssertionError("unreachable")


def region_slug(data, arm_region):
    for region in data.get("regions", []):
        if region["displayName"].replace(" ", "").lower() == arm_region:
            return region["slug"]
    raise Unfillable(f"no calculator region for {arm_region}")


def offer(data, key):
    found = data.get("offers", {}).get(key)
    if found is None:
        raise Unfillable(f"no calculator offer {key}")
    return found


def flat_price(data, key, unit, region):
    price = offer(data, key).get("prices", {}).get(unit, {}).get(region)
    if not price or "value" not in price:
        raise Unfillable(f"offer {key} has no {unit} price in {region}")
    return float(price["value"])


def graduated_total(bands, quantity):
    total = 0.0
    lower = 0.0
    for band in bands:
        upper = float(band["limit"])
        if quantity <= lower:
            break
        total += (min(quantity, upper) - lower) * float(band["price"]["value"])
        lower = upper
    return total


def graduated_bands(data, key, unit, region):
    container = data.get("graduatedOffers", {}).get(key)
    if container is not None:
        bands = container.get(region, {}).get("prices")
    else:
        bands = offer(data, key).get("graduatedPrices", {}).get(unit, {}).get(region, {}).get("prices")
    if not bands:
        raise Unfillable(f"offer {key} has no graduated {unit} price in {region}")
    return bands


def size_key(arm_sku):
    # Standard_D2s_v3 -> d2sv3, the calculator's VM offer naming.
    return arm_sku.removeprefix("Standard_").replace("_", "").lower()


def vm(parts, cache, spot):
    _, arm_sku, arm_region = parts
    url, data = fetch("virtual-machines", cache)
    region = region_slug(data, arm_region)
    key = f"linux-{size_key(arm_sku)}-standard"
    unit = "perhourspot" if spot else "perhour"
    hourly = flat_price(data, key, unit, region)
    return hourly * HOURS, url, f"{key} {unit} {hourly} x {HOURS}"


def managed_disk(parts, cache):
    _, sku, arm_region = parts
    tier, redundancy = sku.split()
    family = {"P": "premiumssd", "E": "standardssd", "S": "standardhdd"}.get(tier[0])
    if family is None:
        raise Unfillable(f"unknown disk tier {tier}")
    url, data = fetch("managed-disks", cache)
    region = region_slug(data, arm_region)
    key = f"{family}-{tier.lower()}-{redundancy.lower()}"
    price = offer(data, key).get("prices", {}).get(region)
    if not price or "value" not in price:
        raise Unfillable(f"offer {key} has no price in {region}")
    monthly = float(price["value"])
    return monthly, url, f"{key} monthly {monthly}, disk only"


def blob(parts, cache):
    _, sku, size, arm_region = parts
    access, redundancy = sku.split()
    gigabytes = float(size.removesuffix("gb"))
    url, data = fetch("storage", cache)
    region = region_slug(data, arm_region)
    key = f"general-purpose-v2-block-blob-structured-{access.lower()}-{redundancy.lower()}"
    found = offer(data, key)
    if "graduatedPrices" in found:
        bands = graduated_bands(data, key, "pergb", region)
        return graduated_total(bands, gigabytes), url, f"{key} graduated pergb x {gigabytes:g} GB"
    rate = flat_price(data, key, "pergb", region)
    return rate * gigabytes, url, f"{key} pergb {rate} x {gigabytes:g} GB"


def app_service_plan(parts, cache):
    _, sku, os_name, arm_region = parts
    url, data = fetch("app-service", cache)
    region = region_slug(data, arm_region)
    size = sku.replace(" ", "").lower()
    families = {"p": "premiumv3"} if size.endswith("v3") else {}
    family = families.get(size[0])
    if family is None:
        raise Unfillable(f"no calculator family mapping for App Service sku {sku}")
    key = f"{os_name}-{family}-{size}-payg"
    price = offer(data, key).get("prices", {}).get(region)
    if not price or "value" not in price:
        raise Unfillable(f"offer {key} has no price in {region}")
    hourly = float(price["value"])
    return hourly * HOURS, url, f"{key} hourly {hourly} x {HOURS}, 1 instance"


def aks_control_plane(parts, cache, tier):
    arm_region = parts[1]
    url, data = fetch("kubernetes-service", cache)
    region = region_slug(data, arm_region)
    if tier == "free":
        slugs = {option["slug"] for option in data.get("slaOptions", [])}
        if "no-sla-free-non-production" not in slugs:
            raise Unfillable("calculator has no Free (no SLA) option")
        if "no-sla-free-non-production" in data.get("offers", {}):
            raise Unfillable("calculator Free option now has an offer; read it before filling")
        return 0.0, url, "slaOption no-sla-free-non-production has no offer, cluster management 0"
    hourly = flat_price(data, "sla", "perhour", region)
    return hourly * HOURS, url, f"sla perhour {hourly} x {HOURS}"


def sql_database(parts, cache):
    _, vcores, size, zone, arm_region = parts
    count = int(vcores.removesuffix("vcore"))
    gigabytes = float(size.removesuffix("gb"))
    url, data = fetch("azure-sql-database", cache)
    region = region_slug(data, arm_region)
    local_key = f"single-vcore-general-purpose-gen5-local-{count}"
    local = flat_price(data, local_key, "perhour", region)
    if zone == "nozr":
        storage = flat_price(data, "single-vcore-general-purpose-local-storage", "pergb", region)
        total = local * HOURS + storage * gigabytes
        return total, url, f"{local_key} {local} x {HOURS} + local-storage {storage} x {gigabytes:g} GB"
    # Calculator: zone redundant compute is the zone offer added to the local
    # offer, and zone redundant storage replaces the local storage rate.
    zone_key = f"single-vcore-general-purpose-gen5-zone-{count}"
    surcharge = flat_price(data, zone_key, "perhour", region)
    storage = flat_price(data, "single-vcore-general-purpose-zone-storage", "pergb", region)
    total = (local + surcharge) * HOURS + storage * gigabytes
    return total, url, (
        f"({local_key} {local} + {zone_key} {surcharge}) x {HOURS}"
        f" + zone-storage {storage} x {gigabytes:g} GB"
    )


def cosmos(parts, cache, autoscale):
    _, throughput, arm_region = parts
    request_units = float(throughput.removesuffix("ru"))
    url, data = fetch("cosmos-db", cache)
    region = region_slug(data, arm_region)
    rate = flat_price(data, "single", "perhour", region)
    if not autoscale:
        return request_units / 100 * rate * HOURS, url, f"single perhour {rate} per 100 RU/s x {HOURS}"
    multiplier = offer(data, "single").get("autoscaleMultiplier")
    if multiplier is None:
        raise Unfillable("cosmos offer single has no autoscaleMultiplier")
    total = request_units / 100 * rate * float(multiplier) * HOURS
    return total, url, (
        f"single perhour {rate} x autoscaleMultiplier {multiplier} per 100 RU/s"
        f" x {HOURS}, 100% utilization of max"
    )


def functions_consumption(parts, cache):
    _, executions, gb_seconds, arm_region = parts
    execution_count = float(executions.removesuffix("exec"))
    duration = float(gb_seconds.removesuffix("gbs"))
    url, data = fetch("functions", cache)
    region = region_slug(data, arm_region)
    # The requests offer is priced per million executions, with the free
    # grant as its first band.
    request_bands = graduated_bands(data, "requests-payg", "", region)
    compute_bands = graduated_bands(data, "compute-payg", "", region)
    total = graduated_total(request_bands, execution_count / 1_000_000) + graduated_total(compute_bands, duration)
    return total, url, "requests-payg per million + compute-payg per GB-s, free grant bands applied"


def quote(case_id, cache):
    parts = case_id.split(":")
    kind = parts[0]
    handlers = {
        "vm_ondemand_linux": lambda: vm(parts, cache, spot=False),
        "vm_spot_linux": lambda: vm(parts, cache, spot=True),
        "managed_disk": lambda: managed_disk(parts, cache),
        "blob": lambda: blob(parts, cache),
        "app_service_plan": lambda: app_service_plan(parts, cache),
        "aks_control_plane_standard": lambda: aks_control_plane(parts, cache, "standard"),
        "aks_control_plane_free": lambda: aks_control_plane(parts, cache, "free"),
        "sql_gp_gen5": lambda: sql_database(parts, cache),
        "cosmos_manual": lambda: cosmos(parts, cache, autoscale=False),
        "cosmos_autoscale": lambda: cosmos(parts, cache, autoscale=True),
        "functions_consumption": lambda: functions_consumption(parts, cache),
    }
    handler = handlers.get(kind)
    if handler is None:
        raise Unfillable(f"no calculator mapping for case kind {kind}")
    return handler()


def main():
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--write", action="store_true", help="write values into the CSV")
    args = parser.parse_args()

    today = datetime.datetime.now(datetime.timezone.utc).date().isoformat()
    with CSV_PATH.open(newline="") as handle:
        reader = csv.DictReader(handle)
        fields = reader.fieldnames
        rows = list(reader)

    cache = {}
    filled = 0
    for row in rows:
        try:
            monthly, url, formula = quote(row["case_id"], cache)
        except Unfillable as reason:
            row["owner_monthly_usd"] = ""
            row["read_on"] = ""
            row["notes"] = f"not filled: {reason}"
            print(f"EMPTY  {row['case_id']}: {reason}")
            continue
        row["owner_monthly_usd"] = f"{monthly:.2f}"
        row["read_on"] = today
        row["notes"] = f"{formula}; source {url.split('?')[0]}"
        filled += 1
        print(f"FILLED {row['case_id']}: {monthly:.2f}  ({formula})")

    print(f"{filled} of {len(rows)} rows filled, read on {today}")
    if args.write:
        with CSV_PATH.open("w", newline="") as handle:
            writer = csv.DictWriter(handle, fieldnames=fields, lineterminator="\n")
            writer.writeheader()
            writer.writerows(rows)
        print(f"wrote {CSV_PATH.relative_to(ROOT)}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
