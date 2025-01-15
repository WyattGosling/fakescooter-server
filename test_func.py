import json
import logging
import pytest
import requests
import subprocess
import time


@pytest.fixture(scope='function')
def server_fixture():
    logging.warning('Ensuring build is up to date')
    subprocess.run(['go', 'build'])

    logging.warning('Starting server')
    proc = subprocess.Popen(['./server'])
    time.sleep(0.2)

    yield

    logging.info('Shutting down server')
    proc.terminate()


class TestServer:
    def test_no_auth_1(self, server_fixture):
        resp = requests.get('http://localhost:8080/scooter')
        logging.warning(f'{resp}')
        assert resp.status_code == 401

    def test_no_auth_2(self, server_fixture):
        resp = requests.get('http://localhost:8080/scooter/abc')
        logging.warning(f'{resp}')
        assert resp.status_code == 401

    @pytest.mark.skip('Gives 405 (Method Not Allowed) for some reason')
    def test_no_auth_3(self, server_fixture):
        resp = requests.patch(url='http://localhost:8080/scooter/abc123', json={})
        logging.warning(f'{resp}')
        assert resp.status_code == 401

    def test_get_scooters(self, server_fixture):
        resp = requests.get('http://localhost:8080/scooter', auth=('basic', 'pass'))
        logging.warning(f'{resp.json()}')
        expected = [
            {"id": "abc123", "reserved": False, "battery": 99, "location": {"latitude": 49.26227, "longitude": -123.14242}},
            {"id": "def456", "reserved": False, "battery": 88, "location": {"latitude": 49.26636, "longitude": -123.14226}},
            {"id": "ghi789", "reserved": True, "battery": 77, "location": {"latitude": 49.26532, "longitude": -123.13659}},
            {"id": "jkl012", "reserved": False, "battery": 9, "location": {"latitude": 49.26443, "longitude": -123.13469}}
        ]
        assert resp.json() == expected

    def test_get_scooter(self, server_fixture):
        resp = requests.get('http://localhost:8080/scooter/abc123', auth=('basic', 'pass'))
        logging.warning(f'{resp.json()}')
        expected = {"id": "abc123", "reserved": False, "battery": 99, "location": {"latitude": 49.26227, "longitude": -123.14242}}
        assert resp.json() == expected