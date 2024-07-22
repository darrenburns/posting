from pathlib import Path
from posting.__main__ import make_posting

COLLECTION = Path(__file__).parent / "sample-collections"
BASE_ENV = COLLECTION / "sample_base.env"
EXTRA_ENV = COLLECTION / "sample_extra.env"

envs = tuple(str(env) for env in [BASE_ENV, EXTRA_ENV])
app = make_posting(collection=COLLECTION, env=envs)
# app.run()
