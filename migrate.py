from alembic import command, config
from alembic.runtime.environment import EnvironmentContext
from alembic.script import ScriptDirectory
from sqlalchemy import engine_from_config, pool
from dotenv import load_dotenv
from urllib.parse import quote_plus
import os
import logging

logging.basicConfig(level=logging.INFO)
load_dotenv()

alembic_cfg = config.Config("alembic.ini")

user = os.getenv("DB_USER")
pwd = quote_plus(os.getenv("DB_PASSWORD"))
host = os.getenv("DB_HOST")
port = os.getenv("DB_PORT")
name = os.getenv("DB_NAME")

db_url = f"postgresql://{user}:{pwd}@{host}:{port}/{name}"
alembic_cfg.set_main_option("sqlalchemy.url", db_url)

script = ScriptDirectory.from_config(alembic_cfg)

print("[migrate] building DATABASE_URL from .env")
print("[migrate] applying upgrade head (if needed)")
try:
    command.upgrade(alembic_cfg, "head")
except Exception as e:
    print(f"[migrate] warning during upgrade: {e}")

print("[migrate] checking for schema changes")

def has_changes():
    def process_revision_directives(context, revision, directives):
        script = directives[0]
        if script.upgrade_ops.is_empty():
            raise SystemExit("[migrate] No changes detected")

    connectable = engine_from_config(
        {"sqlalchemy.url": db_url},
        prefix="sqlalchemy.",
        poolclass=pool.NullPool,
    )

    def do_run_migrations(rev, context):
        context.script = script
        context.connection = connection
        return []

    with connectable.connect() as connection:
        context = EnvironmentContext(
            alembic_cfg,
            script,
            connection=connection,
            target_metadata=None,
            process_revision_directives=process_revision_directives,
            fn=do_run_migrations,
        )
        context.configure(connection=connection)
        with context.begin_transaction():
            context.run_migrations()

try:
    has_changes()
    from pathlib import Path

    if not any(Path("migrations/versions").glob("*.py")):
        print("[migrate] generating initial revision")
        command.revision(alembic_cfg, message="initial", autogenerate=True)
    else:
        print("[migrate] skipping revision: already exists")
except SystemExit as e:
    print(str(e))

print("[migrate] done")
