#!/usr/bin/env bash
# Opt-in queries against the public Retail Prices API.
# Not part of CI. Each filter is the one the quote sends, including currency
# and the Consumption price type, in the builder's field order.
set -euo pipefail

python3 - <<'PY'
import json
import sys
import time
import urllib.error
import urllib.parse
import urllib.request

BASE = "https://prices.azure.com/api/retail/prices"
HOURS = 730
MAX_PAGES = 10


def fetch(filter_text):
    url = BASE + "?" + urllib.parse.urlencode({"$filter": filter_text})
    items = []
    pages = 0
    truncated = False
    while url and pages < MAX_PAGES:
        req = urllib.request.Request(
            url,
            headers={"Accept": "application/json", "User-Agent": "finfocus-live-check"},
        )
        page = None
        for attempt in range(4):
            try:
                with urllib.request.urlopen(req, timeout=90) as resp:
                    page = json.load(resp)
                break
            except urllib.error.HTTPError as exc:
                if exc.code != 429 or attempt == 3:
                    raise
                time.sleep(int(exc.headers.get("Retry-After") or 5))
        if page is None:
            raise RuntimeError("no page")
        items.extend(page.get("Items") or [])
        url = page.get("NextPageLink") or ""
        pages += 1
    if url:
        truncated = True
    return items, pages, truncated


def row(item):
    return (
        f"product={item.get('productName')!r} sku={item.get('skuName')!r} "
        f"meter={item.get('meterName')!r} unit={item.get('unitOfMeasure')!r} "
        f"retail={item.get('retailPrice')} tier={item.get('tierMinimumUnits')} "
        f"end={item.get('effectiveEndDate')!r}"
    )


def word(value, needle):
    return f" {needle} " in f" {(value or '').lower()} "


def first_vm(items, spot):
    for item in items:
        product = (item.get("productName") or "").lower()
        if "windows" in product:
            continue
        name = f"{item.get('skuName') or ''} {item.get('meterName') or ''}".lower()
        if "low priority" in name:
            continue
        is_spot = word(item.get("skuName"), "spot") or word(item.get("meterName"), "spot")
        if is_spot != spot:
            continue
        return item
    return None


def exact(items, meter, sku=None, product=None, unit=None, open_only=False):
    found = []
    for item in items:
        if item.get("meterName") != meter:
            continue
        if sku is not None and item.get("skuName") != sku:
            continue
        if product is not None and item.get("productName") != product:
            continue
        if unit is not None and item.get("unitOfMeasure") != unit:
            continue
        if open_only and str(item.get("effectiveEndDate") or "").strip():
            continue
        found.append(item)
    return found


def marginal(bands, size):
    ordered = sorted(bands, key=lambda item: item.get("tierMinimumUnits") or 0)
    uniq = []
    seen = set()
    for item in ordered:
        minimum = item.get("tierMinimumUnits") or 0
        if minimum in seen:
            continue
        seen.add(minimum)
        uniq.append(item)
    total = 0.0
    for index, item in enumerate(uniq):
        start = item.get("tierMinimumUnits") or 0
        end = float("inf")
        if index + 1 < len(uniq):
            end = uniq[index + 1].get("tierMinimumUnits") or 0
        if size <= start:
            break
        total += (min(size, end) - start) * item["retailPrice"]
    return total


def section(title, filter_text):
    time.sleep(1)
    print(f"\n## {title}")
    print(f"filter: {filter_text}")
    try:
        items, pages, truncated = fetch(filter_text)
    except Exception as exc:  # noqa: BLE001 - one failed query must not hide the rest
        print(f"error: {exc}")
        return None
    print(f"items={len(items)} pages={pages} truncated={truncated}")
    return items


def region_service(region, service, **extra):
    parts = [f"armRegionName eq '{region}'"]
    if "arm" in extra:
        parts.append(f"armSkuName eq '{extra['arm']}'")
    parts.append("currencyCode eq 'USD'")
    parts.append("priceType eq 'Consumption'")
    if "product" in extra:
        parts.append(f"productName eq '{extra['product']}'")
    parts.append(f"serviceName eq '{service}'")
    if "sku" in extra:
        parts.append(f"skuName eq '{extra['sku']}'")
    return " and ".join(parts)


ok = True

vm_filter = region_service("eastus", "Virtual Machines", arm="Standard_B1s")
vm_items = section("Virtual machine on demand", vm_filter)
if vm_items is None:
    ok = False
else:
    chosen = first_vm(vm_items, False)
    if chosen is None:
        print("selected: none")
        ok = False
    else:
        print(row(chosen))
        print(f"month: {chosen['retailPrice']} * {HOURS} = {chosen['retailPrice'] * HOURS}")

spot_filter = region_service("eastus", "Virtual Machines", arm="Standard_D2s_v3")
spot_items = section("Virtual machine Spot", spot_filter)
if spot_items is None:
    ok = False
else:
    chosen = first_vm(spot_items, True)
    if chosen is None:
        print("selected: none")
        ok = False
    else:
        print(row(chosen))
        print(f"month: {chosen['retailPrice']} * {HOURS} = {chosen['retailPrice'] * HOURS}")

disk_filter = region_service(
    "eastus",
    "Storage",
    product="Premium SSD Managed Disks",
    sku="P10 LRS",
)
disk_items = section("Managed disk", disk_filter)
if disk_items is None:
    ok = False
else:
    found = exact(disk_items, "P10 LRS Disk")
    if not found:
        print("selected: none")
        ok = False
    else:
        print(row(found[0]))
        print(f"month: retail {found[0]['retailPrice']} (disk price is already monthly)")

blob_filter = region_service("eastus", "Storage", product="General Block Blob v2", sku="Hot LRS")
blob_items = section("Blob storage", blob_filter)
if blob_items is None:
    ok = False
else:
    bands = [item for item in blob_items if "data stored" in (item.get("meterName") or "").lower()]
    if not bands:
        print("selected: none")
        ok = False
    else:
        for item in bands:
            print(row(item))
        print(f"month at 100 GB: {marginal(bands, 100)}")
        print(f"month at 60000 GB: {marginal(bands, 60000)}")

account_filter = region_service("eastus", "Storage", product="General Block Blob v2")
account_items = section("Storage account", account_filter)
if account_items is None:
    ok = False
else:
    bands = [
        item
        for item in account_items
        if item.get("skuName") == "Hot LRS" and (item.get("meterName") or "").endswith("Data Stored")
    ]
    if not bands:
        print("selected: none")
        ok = False
    else:
        for item in bands:
            print(row(item))
        print(f"month at 100 GB: {marginal(bands, 100)}")

app_filter = region_service("eastus", "Azure App Service")
app_items = section("App Service plan", app_filter)
if app_items is None:
    ok = False
else:
    found = [
        item
        for item in app_items
        if "linux" in (item.get("productName") or "").lower()
        and item.get("unitOfMeasure") == "1 Hour"
        and (item.get("meterName") or "").replace(" ", "").lower() in {"p1v3", "p1v3app"}
    ]
    if not found:
        print("selected: none")
        ok = False
    else:
        print(row(found[0]))
        print(f"month: {found[0]['retailPrice']} * {HOURS} = {found[0]['retailPrice'] * HOURS}")

fn_filter = region_service("eastus", "Functions")
fn_items = section("Function app", fn_filter)
if fn_items is None:
    ok = False
else:
    names = {"Standard Execution Time", "Standard Total Executions"}
    found = [item for item in fn_items if item.get("meterName") in names and (item.get("retailPrice") or 0) > 0]
    if len(found) < 2:
        print("selected: none")
        ok = False
    for item in found:
        print(row(item))
    print("executions are billed per 10; a positive row replaces the zero grant row")

aks_filter = region_service("eastus", "Azure Kubernetes Service")
aks_items = section("AKS", aks_filter)
if aks_items is None:
    ok = False
else:
    standard = exact(aks_items, "Standard Uptime SLA", unit="1 Hour")
    free = exact(aks_items, "FreeTierInfrastructureCost Uptime SLA", unit="1 Hour", open_only=True)
    if not standard or not free:
        print("selected: none")
        ok = False
    for item in standard[:1] + free[:1]:
        print(row(item))
        print(f"month: {item['retailPrice']} * {HOURS} = {item['retailPrice'] * HOURS}")

sql_compute = region_service(
    "eastus",
    "SQL Database",
    product="SQL Database Single/Elastic Pool General Purpose - Compute Gen5",
)
compute_items = section("SQL compute", sql_compute)
if compute_items is None:
    ok = False
else:
    found = exact(compute_items, "vCore", sku="2 vCore", unit="1 Hour")
    if not found:
        print("selected: none")
        ok = False
    else:
        print(row(found[0]))
        print(f"month: {found[0]['retailPrice']} * {HOURS} = {found[0]['retailPrice'] * HOURS}")

sql_storage = region_service(
    "eastus",
    "SQL Database",
    product="SQL Database Single/Elastic Pool General Purpose - Storage",
)
storage_items = section("SQL storage", sql_storage)
if storage_items is None:
    ok = False
else:
    found = exact(storage_items, "General Purpose Data Stored", unit="1 GB/Month")
    if not found:
        print("selected: none")
        ok = False
    else:
        print(row(found[0]))
        print(f"month at 100 GB: {found[0]['retailPrice']} * 100 = {found[0]['retailPrice'] * 100}")

cosmos_filter = region_service("eastus", "Azure Cosmos DB")
cosmos_items = section("Cosmos DB", cosmos_filter)
if cosmos_items is None:
    ok = False
else:
    ru = exact(cosmos_items, "100 RU/s", sku="RUs", product="Azure Cosmos DB")
    stored = exact(cosmos_items, "Data Stored", sku="RUs", product="Azure Cosmos DB", unit="1 GB/Month")
    request_units = [
        item
        for item in cosmos_items
        if item.get("meterName") == "1M RUs" and item.get("productName") == "Azure Cosmos DB serverless"
    ]
    if not ru:
        print("selected: none for 100 RU/s")
        ok = False
    else:
        print(row(ru[0]))
        print(f"month at 400 RU/s: (400/100) * {ru[0]['retailPrice']} * {HOURS} = {4 * ru[0]['retailPrice'] * HOURS}")
    for item in stored[:1]:
        print(row(item))
        print(f"month at 10 GB: {item['retailPrice']} * 10 = {item['retailPrice'] * 10}")
    if not request_units:
        print("request-unit product: no 1M RUs row")
    else:
        print(row(request_units[0]))
        print("request-unit product has no storage meter in this selection")

lb_east = region_service("eastus", "Load Balancer")
east_items = section("Load Balancer regional", lb_east)
if east_items is None:
    ok = False
else:
    print(f"regional rows: {len(east_items)}")

lb_global = region_service("Global", "Load Balancer")
global_items = section("Load Balancer price region", lb_global)
if global_items is None:
    ok = False
else:
    included = exact(global_items, "Standard Included LB Rules and Outbound Rules", unit="1 Hour")
    if not included:
        print("selected: none")
        ok = False
    else:
        print(row(included[0]))
        print(f"month: {included[0]['retailPrice']} * {HOURS} = {included[0]['retailPrice'] * HOURS}")

sys.exit(0 if ok else 1)
PY
