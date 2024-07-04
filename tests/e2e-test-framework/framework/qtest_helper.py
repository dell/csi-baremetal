import json
import requests

# Swagger
# https://qtest.dev.tricentis.com/


class QTestHelper():
    def __init__(self, qtest_token):

        self.api_base_url = "https://qtest.gtie.dell.com/api/v3"
        self.project_id = 367  # CMO project

        # maps from pytest outcome to qTest status code
        self.status_codes = {
            "passed": 601,
            "failed": 602,
            "skipped": 603  # 603 - incomplete
        }

        self.access_token = qtest_token

        self.headers = {
            "Authorization": f"Bearer {self.access_token}",
            "Content-Type": "application/json"
        }

        self.timeout = 10

    def link_test_case_to_requirement(self, jira_id, test_case_pid):
        requirement_id = self.get_requirement_id_by_jira_id(jira_id)
        req_link_endpoint = f"{self.api_base_url}/projects/{self.project_id}/requirements/{requirement_id}/link?type=test-cases"

        test_case_id = self.get_test_case_pid_by_id(test_case_pid)
        body = [test_case_id]

        response = requests.post(req_link_endpoint, headers=self.headers, data=json.dumps(body), timeout=self.timeout)
        response.raise_for_status()

        print(f"\tTest cases {test_case_pid} [{test_case_id}] linked to requirement {jira_id} [{requirement_id}] successfully.")

    def get_test_case_pid_by_id(self, test_case_pid):
        test_cases_endpoint = f"{self.api_base_url}/projects/{self.project_id}/test-cases/{test_case_pid}"
        response = requests.get(test_cases_endpoint, headers=self.headers, timeout=self.timeout)
        response.raise_for_status()

        return response.json()['id']

    def get_requirement_id_by_jira_id(self, jira_id):
        search_endpoint = f"{self.api_base_url}/projects/{self.project_id}/search"

        payload = {
            "object_type": "requirements",
            "fields": [
                "id"
            ],
            "query": f"'name' ~ '{jira_id}'"
        }

        response = requests.post(search_endpoint, headers=self.headers, json=payload, timeout=self.timeout)
        response.raise_for_status()

        return response.json()['items'][0]['id']

    def get_tests_from_suite(self, test_suite_id):
        test_runs_endpoint = f"{self.api_base_url}/projects/{self.project_id}/test-runs/?parentId={test_suite_id}&parentType=test-suite"

        response = requests.get(test_runs_endpoint, headers=self.headers, timeout=self.timeout)
        response.raise_for_status()

        tests_in_suite = {}
        for test_run in response.json()['items']:
            tests_in_suite[test_run['testCaseId']] = test_run['id']

        return tests_in_suite

    def add_test_run_to_test_suite(self, test_case_pid, test_suite_id):
        get_test_case_endpoint = f"{self.api_base_url}/projects/{self.project_id}/test-cases/{test_case_pid}"

        response = requests.get(get_test_case_endpoint, headers=self.headers, timeout=self.timeout)
        response.raise_for_status()

        add_test_case_to_test_suite_endpoint = f"{self.api_base_url}/projects/{self.project_id}/test-runs?parentId={test_suite_id}&parentType=test-suite"

        payload = {
            "name": response.json()['name'],
            "test_case": {
                "id": test_case_pid,
            }
        }
        response = requests.post(add_test_case_to_test_suite_endpoint, headers=self.headers, data=json.dumps(payload), timeout=self.timeout)
        response.raise_for_status()

        return response.json()['id']

    def update_test_run_status_in_test_suite(self, test_run_id, status, test_start_date, test_end_date):
        update_test_logs_endpoint = f"{self.api_base_url}/projects/{self.project_id}/test-runs/{test_run_id}/test-logs"

        payload = {
            "exe_start_date": test_start_date.isoformat().split(".")[0] + "+00:00",
            "exe_end_date": test_end_date.isoformat().split(".")[0] + "+00:00",
            "status": {
                "id": self.status_codes[status]
            },
        }

        response = requests.post(update_test_logs_endpoint, headers=self.headers, data=json.dumps(payload), timeout=self.timeout)
        response.raise_for_status()
