# Nigeria geography catalog

36 states, FCT and 774 local government areas/area councils. IDs are application identifiers retained from the source snapshot, not official government codes. Labels and explicit legacy aliases share one embedded catalog for server forms, discovery and the database migration.

Adapted from [Open Admin Data](https://github.com/open-admin-data/nigeria-administrative-divisions), licensed under [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/); see DATA-LICENSE.txt. The snapshot was reduced to identifiers, labels and parent relationships. Outdated labels/typos were corrected against the [NILDS Constitution, including the Fifth Alterations (2023)](https://ir.nilds.gov.ng/handle/123456789/1729), First Schedule, Parts I/II: Afikpo, Edda, Yewa North/South, Ghari, and spelling corrections. Original source labels remain explicit aliases. FCT's six entries are area councils.

Only exact normalized labels, IDs and documented aliases resolve. Normalization ignores ASCII case, spacing and punctuation. Neighborhoods are never inferred to be LGAs. Unmatched legacy areas remain unchanged with a null area ID; their recognized state may still resolve. Archived accounts retain their historical text and are excluded from backfill. Owners must choose a canonical pair when submitting a profile edit.

The SQL catalog is an immutable copy of this snapshot. A future catalog change requires a new migration. Tests compare the SQL catalog with the embedded catalog.
