import logging
from datetime import datetime
import pytest
from framework.description_plugin import DescriptionPlugin
from framework.qtest_helper import QTestHelper
from framework.propagating_thread import PropagatingThread
import re

@pytest.hookimpl(trylast=True)
def pytest_configure(config):
    terminal_reporter = config.pluginmanager.getplugin('terminalreporter')
    config.pluginmanager.register(DescriptionPlugin(terminal_reporter), 'testdescription')

    # Configure log file logging
    log_file_suffix = '{:%Y_%m_%d_%H%M%S}.log'.format(datetime.now())
    log_file = f'logs/pytest_{log_file_suffix}'
    file_handler = logging.FileHandler(log_file)
    file_handler.setLevel(logging.INFO)
    file_formatter = logging.Formatter("%(asctime)s [%(levelname)s] %(message)s")
    file_handler.setFormatter(file_formatter)
    logging.getLogger().addHandler(file_handler)

    pytest.qtest_helper = QTestHelper(config.getoption("--qtest_token")) if config.getoption("--qtest_token") else None

    pytest.tests_in_suite = {}
    pytest.threads = []

def pytest_addoption(parser):
    parser.addoption("--login", action="store", default="", help="Login")
    parser.addoption("--password", action="store", default="", help="Password")
    parser.addoption("--namespace", action="store", default="atlantic", help="Namespace")
    parser.addoption("--qtest_token", action="store", default="", help="qTest Token")
    parser.addoption("--qtest_test_suite", action="store", default="", help="qTest Test Suite ID")

def pytest_collection_modifyitems(config):
    qtest_token = config.getoption("--qtest_token")
    test_suite_id = config.getoption("--qtest_test_suite")
    if qtest_token == '':
        logging.warning(" [qTest] Test Cases are not updated with requirements.")
        logging.warning(" [qTest] Test suite will not be updated.")
        return

    if test_suite_id == '':
        logging.warning(" [qTest] Test suite will not be updated.")
        return

    pytest.tests_in_suite = pytest.qtest_helper.get_tests_from_suite(test_suite_id)
    logging.info(f" [qTest] Test suite {test_suite_id} will be updated.")

def pytest_sessionfinish():
    if len(pytest.threads) == 0:
        return

    suite_failed = False
    for thread in pytest.threads:
        try:
            thread.join()
        except Exception:
            suite_failed = True

    logging.info("[qTest] Summary")
    for thread in pytest.threads:
        if thread.has_failed():
            logging.error(f"[qTest] {thread.test_name} {thread.get_target_name()} failed: {thread.exc}")
        if not thread.has_failed():
            logging.info(f"[qTest] {thread.test_name} {thread.get_target_name()} success.")

    assert not suite_failed, "One or more threads failed"

@pytest.fixture(scope="session")
def vm_user(request):
    return request.config.getoption("--login")

@pytest.fixture(scope="session")
def vm_cred(request):
    return request.config.getoption("--password")

@pytest.fixture(scope="session")
def namespace(request):
    return request.config.getoption("--namespace")

@pytest.fixture(scope="function", autouse=True)
def link_requirements_in_background(request):
    if pytest.qtest_helper is not None:
        requirements_thread = PropagatingThread(target=link_requirements, args=(request,), test_name=request.node.name)
        requirements_thread.start()
        pytest.threads.append(requirements_thread)

def link_requirements(request):
    for marker in request.node.iter_markers():
            if marker.name == "requirements":
                logging.info(f" [qTest] Test function {request.node.name} is associated with requirement: {marker.args}.")
                test_case_pid, requirement_ids = marker.args
                for requirement_id in requirement_ids:
                    pytest.qtest_helper.link_test_case_to_requirement(requirement_id, test_case_pid)
                return
    logging.info(f"[qTest] Test function {request.node.name} is missing requirements marker.")

@pytest.hookimpl(tryfirst=True, hookwrapper=True)
def pytest_runtest_makereport(item):
    report = (yield).get_result()
    setattr(item, 'report', report)


@pytest.fixture(scope="function", autouse=True)
def update_test_results_in_background(request):
    test_suite_id = request.config.getoption("--qtest_test_suite")

    # skip execution if missing qtest params
    if test_suite_id == '' or pytest.qtest_helper is None:
        yield
        return

    test_start_date = datetime.now()
    yield
    test_end_date = datetime.now()

    update_thread = PropagatingThread(target=update_test_result,
                                      args=(request, test_start_date, test_end_date),
                                      test_name=request.node.name)
    update_thread.start()
    pytest.threads.append(update_thread)

def update_test_result(request, test_start_date, test_end_date):
    test_name = request.node.name
    test_suite_id = request.config.getoption("--qtest_test_suite")

    match = re.match(r"test_(\d+)", test_name)

    if not match:
        raise Exception(f"Test doesn't contain a valid ID")

    test_case = f"TC-{match.groups()[0]}"
    test_run_id = None
    test_case_id = pytest.qtest_helper.get_test_case_pid_by_id(test_case)

    if test_case_id not in pytest.tests_in_suite:
        test_run_id = pytest.qtest_helper.add_test_run_to_test_suite(test_case_id, test_suite_id)
        logging.info(f"[qTest] Added test case {test_case} to test suite {test_suite_id}")
    else:
        test_run_id = pytest.tests_in_suite[test_case_id]

    pytest.qtest_helper.update_test_run_status_in_test_suite(test_run_id, request.node.report.outcome, test_start_date, test_end_date)

    logging.info(f"[qTest] Updated test run {test_run_id} with status {request.node.report.outcome}")

