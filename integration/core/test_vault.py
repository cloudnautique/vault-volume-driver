import pytest
import requests


token_request_data = {
    'policies': 'default'
}


def _get_token_create_url():
    return "http://127.0.0.1:8080/v1-vault-driver/tokens"


def _post(url, data):
    resp = requests.post(url, json=data, timeout=10.0)
    assert resp.status_code == 200
    return resp


@pytest.fixture
def token_request(scope="function"):
    return token_request_data


def test_request_token(token_request):
    expected_keys = ["accessor", "actions", "token", "type", "links"]
    response = _post(_get_token_create_url(), token_request)

    print(response.json())
    expected_keys.sort()
    keys = response.json().keys()
    keys.sort()

    assert expected_keys == keys
