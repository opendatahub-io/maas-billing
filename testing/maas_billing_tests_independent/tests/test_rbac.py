from conftest import bearer

def test_user_cannot_list_admin_keys(http, base_url, maas_key):
    r = http.get(f"{base_url}/v1/keys", headers=bearer(maas_key))
    if r.status_code != 404:
        assert r.status_code in (401, 403)
