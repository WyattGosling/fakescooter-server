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
    time.sleep(0.3)

    yield

    logging.info('Shutting down server')
    proc.terminate()
    logging.error(proc.stdout)
    logging.error(proc.stderr)


class TestServer:
    def test_no_auth_1(self, server_fixture):
        resp = requests.get('http://localhost:8080/scooter')
        logging.warning(f'{resp}')
        assert resp.status_code == requests.codes.unauthorized

    def test_no_auth_2(self, server_fixture):
        resp = requests.get('http://localhost:8080/scooter/abc')
        logging.warning(f'{resp}')
        assert resp.status_code == requests.codes.unauthorized

    def test_no_auth_3(self, server_fixture):
        resp = requests.patch(url='http://localhost:8080/scooter/abc123', json={})
        logging.warning(f'{resp}')
        assert resp.status_code == requests.codes.unauthorized

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

    def test_patch_scooter_id(self, server_fixture):
        data = json.dumps({"id": 99999})
        resp = requests.patch('http://localhost:8080/scooter/abc123', data=data, auth=('basic', 'pass'))
        logging.warning(f'{resp.content}')
        assert resp.status_code == requests.codes.created

    def test_patch_scooter_reservation(self, server_fixture):
        data = json.dumps({'reserved': True})
        resp = requests.patch('http://localhost:8080/scooter/abc123', data=data, auth=('basic', 'pass'))
        logging.warning(f'{resp.content}')
        assert resp.status_code == requests.codes.created

        resp = requests.get('http://localhost:8080/scooter/abc123', auth=('basic', 'pass'))
        logging.warning(f'{resp.json()}')
        expected = {'id': 'abc123', 'reserved': True, 'battery': 99, 'location': {'latitude': 49.26227, 'longitude': -123.14242}}
        assert resp.json() == expected

    def test_patch_scooter_battery(self, server_fixture):
        data = json.dumps({'battery': -1})
        resp = requests.patch('http://localhost:8080/scooter/abc123', data=data, auth=('basic', 'pass'))
        assert resp.status_code == requests.codes.bad_request

        data = json.dumps({'battery': 101})
        resp = requests.patch('http://localhost:8080/scooter/abc123', data=data, auth=('basic', 'pass'))
        assert resp.status_code == requests.codes.bad_request

        data = json.dumps({'battery': 25})
        resp = requests.patch('http://localhost:8080/scooter/abc123', data=data, auth=('basic', 'pass'))
        logging.warning(f'{resp.content}')
        assert resp.status_code == requests.codes.created
        expected = {'id': 'abc123', 'reserved': False, 'battery': 25, 'location': {'latitude': 49.26227, 'longitude': -123.14242}}
        assert resp.json() == expected

    def test_patch_scoot_location(self, server_fixture):
        data = json.dumps({'location': {'latitude': -181, 'longitude': -123.14242}})
        resp = requests.patch('http://localhost:8080/scooter/abc123', data=data, auth=('basic', 'pass'))
        assert resp.status_code == requests.codes.bad_request
        data = json.dumps({'location': {'latitude': 49.26227, 'longitude': 700}})
        resp = requests.patch('http://localhost:8080/scooter/abc123', data=data, auth=('basic', 'pass'))
        assert resp.status_code == requests.codes.bad_request

        data = json.dumps({'location': {'latitude': 99, 'longitude': -99}})
        resp = requests.patch('http://localhost:8080/scooter/abc123', data=data, auth=('basic', 'pass'))
        logging.warning(f'{resp.content}')
        assert resp.status_code == requests.codes.created
        expected = {'id': 'abc123', 'reserved': False, 'battery': 99, 'location': {'latitude': 99, 'longitude': -99}}
        assert resp.json() == expected