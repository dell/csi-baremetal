import json
import logging
import requests

# Swagger
# https://qtest.dev.tricentis.com/


class QTestHelper:
    @classmethod
    def __init__(cls, qtest_token, bundle_version=""):

        cls.api_base_url = "https://qtest.gtie.dell.com/api/v3"
        cls.project_id = 367  # CMO project
        cls.bundle_version = bundle_version
        cls.default_timeout = 10
        cls.max_retries = 2

        # maps from pytest outcome to qTest status code
        cls.status_codes = {
            "passed": 601,
            "failed": 602,
            "skipped": 603,  # 603 - incomplete
        }

        cls.access_token = qtest_token

        cls.headers = {
            "Authorization": f"Bearer {cls.access_token}",
            "Content-Type": "application/json",
        }

    def link_test_case_to_requirement(self, jira_id, test_case_pid):
        logging.info(f"[qTest] Link test case {test_case_pid} to requirement")
        requirement_id = self.get_requirement_id_by_jira_id(jira_id)
        req_link_endpoint = f"{self.api_base_url}/projects/{self.project_id}/requirements/{requirement_id}/link?type=test-cases"

        test_case_id = self.get_test_case_pid_by_id(test_case_pid)
        body = [test_case_id]

        exception = None
        for _ in range(self.max_retries):
            try:
                response = requests.post(
                    req_link_endpoint,
                    headers=self.headers,
                    data=json.dumps(body),
                    timeout=self.default_timeout,
                )
                response.raise_for_status()
                logging.info(
                    f"[qTest] Test cases {test_case_pid} [{test_case_id}] linked to requirement {jira_id} [{requirement_id}] successfully."
                )
                return
            except requests.HTTPError as exc:
                exception = exc
                logging.warning(
                    f"[qTest] Failed to link test case {test_case_pid} to requirement {jira_id}. Retrying..."
                )

        raise exception

    def get_test_case_pid_by_id(self, test_case_pid):
        logging.info(f"[qTest] Get test case {test_case_pid} by id")
        test_cases_endpoint = f"{self.api_base_url}/projects/{self.project_id}/test-cases/{test_case_pid}"

        exception = None
        for _ in range(self.max_retries):
            try:
                response = requests.get(
                    test_cases_endpoint,
                    headers=self.headers,
                    timeout=self.default_timeout,
                )
                response.raise_for_status()

                logging.info(
                    f"[qTest] Get test case {test_case_pid} by id completed."
                )
                return response.json()["id"]
            except requests.HTTPError as exc:
                exception = exc
                logging.warning(
                    f"[qTest] Failed to get test case {test_case_pid} by id. Retrying..."
                )

        raise exception

    def get_requirement_id_by_jira_id(self, jira_id):
        logging.info(f"[qTest] Get requirement id by jira id {jira_id}")
        search_endpoint = (
            f"{self.api_base_url}/projects/{self.project_id}/search"
        )

        payload = {
            "object_type": "requirements",
            "fields": ["id"],
            "query": f"'name' ~ '{jira_id}'",
        }

        exception = None
        for _ in range(self.max_retries):
            try:
                response = requests.post(
                    search_endpoint,
                    headers=self.headers,
                    json=payload,
                    timeout=self.default_timeout,
                )
                response.raise_for_status()

                logging.info(
                    f"[qTest] Get requirement id by jira id {jira_id} completed."
                )
                return response.json()["items"][0]["id"]
            except requests.HTTPError as exc:
                exception = exc
                logging.warning(
                    f"[qTest] Failed to get requirement id by jira id {jira_id}. Retrying..."
                )

        raise exception

    def get_tests_from_suite(self, test_suite_id):
        logging.info(f"[qTest] Get tests from test suite {test_suite_id}")
        test_runs_endpoint = f"{self.api_base_url}/projects/{self.project_id}/test-runs/?parentId={test_suite_id}&parentType=test-suite"

        exception = None
        for _ in range(self.max_retries):
            try:
                response = requests.get(
                    test_runs_endpoint,
                    headers=self.headers,
                    timeout=self.default_timeout,
                )
                response.raise_for_status()

                tests_in_suite = {}
                for test_run in response.json()["items"]:
                    tests_in_suite[test_run["testCaseId"]] = test_run["id"]

                logging.info(
                    f"[qTest] Get tests from test suite {test_suite_id} completed."
                )
                return tests_in_suite
            except requests.HTTPError as exc:
                exception = exc
                logging.warning(
                    f"[qTest] Failed to get tests from test suite {test_suite_id}. Retrying..."
                )

        raise exception

    def add_test_run_to_test_suite(self, test_case_pid, test_suite_id):
        logging.info(
            f"[qTest] Add test case {test_case_pid} to test suite {test_suite_id}"
        )
        get_test_case_endpoint = f"{self.api_base_url}/projects/{self.project_id}/test-cases/{test_case_pid}"

        exception = None
        for _ in range(self.max_retries):
            try:
                response = requests.get(
                    get_test_case_endpoint,
                    headers=self.headers,
                    timeout=self.default_timeout,
                )
                response.raise_for_status()

                logging.info(
                    f"[qTest] Test case {test_case_pid} added to test suite {test_suite_id}"
                )
                break
            except requests.HTTPError as exc:
                exception = exc
                logging.warning(
                    f"[qTest] Failed to add test case {test_case_pid} to test suite {test_suite_id}. Retrying..."
                )

        if exception is not None:
            raise exception

        add_test_case_to_test_suite_endpoint = f"{self.api_base_url}/projects/{self.project_id}/test-runs?parentId={test_suite_id}&parentType=test-suite"

        payload = {
            "name": response.json()["name"],
            "test_case": {
                "id": test_case_pid,
            },
        }

        for _ in range(self.max_retries):
            try:
                response = requests.post(
                    add_test_case_to_test_suite_endpoint,
                    headers=self.headers,
                    data=json.dumps(payload),
                    timeout=self.default_timeout,
                )
                response.raise_for_status()
                logging.info(
                    f"[qTest] Test case {test_case_pid} added to test suite {test_suite_id}"
                )
                return response.json()["id"]
            except requests.HTTPError as exc:
                exception = exc
                logging.warning(
                    f"[qTest] Failed to add test case {test_case_pid} to test suite {test_suite_id}. Retrying..."
                )

        raise exception

    def update_test_run_status_in_test_suite(
        self, test_run_id, status, test_start_date, test_end_date
    ):
        logging.info(
            f"[qTest] Update test run {test_run_id} with status {status}"
        )
        update_test_logs_endpoint = f"{self.api_base_url}/projects/{self.project_id}/test-runs/{test_run_id}/test-logs"

        payload = {
            "exe_start_date": test_start_date.isoformat().split(".")[0]
            + "+00:00",
            "exe_end_date": test_end_date.isoformat().split(".")[0] + "+00:00",
            "status": {"id": self.status_codes[status]},
        }

        if self.bundle_version != "":
            payload["properties"] = [
                {
                    "field_id": 103442,  # bundle version
                    "field_value": self.bundle_version,
                }
            ]

        exception = None
        for _ in range(self.max_retries):
            try:
                response = requests.post(
                    update_test_logs_endpoint,
                    headers=self.headers,
                    data=json.dumps(payload),
                    timeout=self.default_timeout,
                )
                response.raise_for_status()

                logging.info(
                    f"[qTest] Test run {test_run_id} status updated successfully."
                )
                return
            except requests.HTTPError as exc:
                exception = exc
                logging.warning(
                    f"[qTest] Failed to update test run {test_run_id} status. Retrying..."
                )

        raise exception
