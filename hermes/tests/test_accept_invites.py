"""Tests for the hermes worker's Matrix invite acceptance at startup."""

from __future__ import annotations

import json as json_mod
import urllib.request

from hermes_worker.config import WorkerConfig
from hermes_worker.worker import Worker


def _config(tmp_path) -> WorkerConfig:
    return WorkerConfig(
        worker_name="alice",
        minio_endpoint="http://minio:9000",
        minio_access_key="alice",
        minio_secret_key="secret",
        minio_bucket="hiclaw",
        install_dir=tmp_path,
    )


class _FakeResponse:
    def __init__(self, data):
        self._data = data

    def __enter__(self):
        return self

    def __exit__(self, *_):
        return False

    def read(self):
        return json_mod.dumps(self._data).encode()


def test_accept_matrix_invites_joins_each_pending(tmp_path, monkeypatch):
    sent = []

    def fake_urlopen(req, timeout=None):
        url = req.full_url
        sent.append((req.get_method(), url))
        if "/sync" in url:
            return _FakeResponse(
                {"rooms": {"invite": {"!dm:test": {}, "!team:test": {}}}}
            )
        if "/join/" in url:
            return _FakeResponse({"room_id": "joined"})
        return _FakeResponse({})

    monkeypatch.setattr(urllib.request, "urlopen", fake_urlopen)

    worker = Worker(_config(tmp_path))
    worker._accept_matrix_invites(
        {"channels": {"matrix": {"homeserver": "http://matrix:6167", "accessToken": "tok"}}}
    )

    methods = [m for m, _ in sent]
    urls = [u for _, u in sent]
    assert methods[0] == "GET" and "/sync" in urls[0]
    join_posts = [u for m, u in sent if m == "POST" and "/join/" in u]
    assert len(join_posts) == 2  # one join per pending invite


def test_accept_matrix_invites_noop_when_none_pending(tmp_path, monkeypatch):
    sent = []

    def fake_urlopen(req, timeout=None):
        sent.append((req.get_method(), req.full_url))
        return _FakeResponse({"rooms": {"invite": {}}})

    monkeypatch.setattr(urllib.request, "urlopen", fake_urlopen)

    worker = Worker(_config(tmp_path))
    worker._accept_matrix_invites(
        {"channels": {"matrix": {"homeserver": "http://matrix:6167", "accessToken": "tok"}}}
    )

    # Only the sync call, no joins.
    assert all("/join/" not in u for _, u in sent)


def test_accept_matrix_invites_skips_without_token(tmp_path, monkeypatch):
    called = []
    monkeypatch.setattr(
        urllib.request, "urlopen",
        lambda *a, **k: called.append(1),
    )
    worker = Worker(_config(tmp_path))
    worker._accept_matrix_invites(
        {"channels": {"matrix": {"homeserver": "http://matrix:6167"}}}  # no token
    )
    assert not called
