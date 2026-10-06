# Choosing a VM size and region

When you run `serverku init` interactively, serverku now shows a **selectable
list** of regions and VM sizes instead of a free-text box. Where possible the
list is pulled **live** from your provider (with prices); otherwise it falls
back to the built-in suggestions below. If you already know the slug you want,
you can also pass it non-interactively with `--region` / `--size`.

## How the picker gets its options

- **DigitalOcean:** if a token is configured (`serverku setup digitalocean` or
  `DIGITALOCEAN_TOKEN`), the picker lists the real regions and the basic droplet
  sizes available in the chosen region, with live monthly prices. Pick the
  region first — the size list updates to what that region offers.
- **GCP / no credentials / offline:** the picker uses the curated static list
  below. Edit the generated `~/.serverku/projects/<name>.yaml` afterward if you
  need a size or region that isn't listed.

`--size` / `--region` passed non-interactively for DigitalOcean are validated
against the live catalog, so a wrong slug fails immediately with suggestions
instead of a confusing error at create time.

## DigitalOcean — common basic droplet sizes

| Slug | vCPU | RAM | Disk | ~Price/mo |
|------|------|-----|------|-----------|
| `s-1vcpu-512mb-10gb` | 1 | 512 MB | 10 GB | $4 |
| `s-1vcpu-1gb` | 1 | 1 GB | 25 GB | $6 |
| `s-1vcpu-2gb` | 1 | 2 GB | 50 GB | $12 |
| `s-2vcpu-2gb` | 2 | 2 GB | 60 GB | $18 |
| `s-2vcpu-4gb` | 2 | 4 GB | 80 GB | $24 |
| `s-4vcpu-8gb` | 4 | 8 GB | 160 GB | $48 |

Common regions: `sgp1` (Singapore), `nyc1`/`nyc3` (New York), `ams3`
(Amsterdam), `fra1` (Frankfurt), `lon1` (London), `sfo3` (San Francisco),
`blr1` (Bangalore), `syd1` (Sydney), `tor1` (Toronto).

Rule of thumb: WordPress/MySQL and most small stacks want **at least
`s-1vcpu-1gb`** (512 MB is very tight and can OOM).

Full, always-current list:
<https://slugs.do-api.dev/> · pricing: <https://www.digitalocean.com/pricing/droplets>

## GCP — common machine types

| Slug | vCPU | RAM | Notes |
|------|------|-----|-------|
| `e2-micro` | shared | 1 GB | cheapest; bursty |
| `e2-small` | shared | 2 GB | light workloads |
| `e2-medium` | shared | 4 GB | good default |
| `e2-standard-2` | 2 | 8 GB | steady 2 vCPU |
| `n2-standard-2` | 2 | 8 GB | newer gen |

Regions are picked as `region` + `zone` (e.g. region `asia-southeast1`, zone
`asia-southeast1-b`). Machine-type availability varies by zone.

Full, always-current list:
machine types <https://cloud.google.com/compute/docs/machine-resource> ·
regions/zones <https://cloud.google.com/compute/docs/regions-zones> ·
pricing <https://cloud.google.com/compute/all-pricing>

## Notes

- The boot disk that ships with the size (the DigitalOcean `Disk` column /
  GCP's default) is **separate** from serverku's persistent `storage` volume.
  Enable `storage` to keep data across `serverku down`.
- `spot` is GCP-only. DigitalOcean has no spot/preemptible tier.
