import logging
from datetime import datetime
import pytest
from framework.test_description_plugin import TestDescriptionPlugin
from wiremock.testing.testcontainer import wiremock_container
from wiremock.constants import Config
from framework.qtest_helper import QTestHelper
from framework.docker_helper import Docker
import re
from datetime import datetime
from framework.propagating_thread import PropagatingThread

@pytest.mark.trylast
def pytest_configure(config):
    terminal_reporter = config.pluginmanager.getplugin('terminalreporter')
    config.pluginmanager.register(TestDescriptionPlugin(terminal_reporter), 'testdescription')

    # Configure log file logging
    log_file_suffix = '{:%Y_%m_%d_%H%M%S}.log'.format(datetime.now())
    log_file = f'logs/pytest_{log_file_suffix}'
    file_handler = logging.FileHandler(log_file)
    file_handler.setLevel(logging.INFO)
    file_formatter = logging.Formatter("%(asctime)s [%(levelname)s] %(message)s")
    file_handler.setFormatter(file_formatter)
    logging.getLogger().addHandler(file_handler)

    pytest.qtest_helper = QTestHelper(config.getoption("--qtest_token"), config.getoption("--cmo_bundle_version")) if config.getoption("--qtest_token") else None

    pytest.tests_in_suite = {}
    pytest.threads = []

def pytest_addoption(parser):
    parser.addoption("--login", action="store", default="", help="Login")
    parser.addoption("--password", action="store", default="", help="Password")
    parser.addoption("--namespace", action="store", default="atlantic", help="Namespace")
    parser.addoption("--hosts", action="store", default=[], help="Hosts")
    parser.addoption("--qtest_token", action="store", default="", help="qTest Token")
    parser.addoption("--ansible_server", action="store", default="", help="Server")
    parser.addoption("--qtest_test_suite", action="store", default="", help="qTest Test Suite ID")
    parser.addoption("--cmo_bundle_version", action="store", default="", help="Version of CMO bundle")

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

    for thread in pytest.threads:
        thread.join()

    logging.info("[qTest] Summary")
    for thread in pytest.threads:
        if thread.has_failed():
            logging.error(f"[qTest] {thread.test_name} {thread.get_target_name()} failed: {thread.exc}")
        else:
            logging.info(f"[qTest] {thread.test_name} {thread.get_target_name()} success.")

@pytest.fixture(scope="session")
def vm_user(request):
    return request.config.getoption("--login")

@pytest.fixture(scope="session")
def vm_cred(request):
    return request.config.getoption("--password")

@pytest.fixture(scope="session")
def namespace(request):
    return request.config.getoption("--namespace")

@pytest.fixture(scope="session")
def hosts(request):
    return request.config.getoption("--hosts")

@pytest.fixture(scope="session")
def ansible_server(request):
    return request.config.getoption("--ansible_server")

@pytest.fixture(scope="session")
def wire_mock():
    if not Docker.is_docker_running():
        pytest.skip('Docker is not running. Please start docker.')
    with wiremock_container(image="asdrepo.isus.emc.com:9042/wiremock:2.35.1-1", verify_ssl_certs=False) as wire_mock:
        Config.base_url = wire_mock.get_url("__admin")
        Config.requests_verify = False
        yield wire_mock

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

    if report.outcome == 'skipped' and pytest.qtest_helper is not None and item.config.getoption("--qtest_test_suite") != '':
        update_thread = PropagatingThread(target=update_test_result,
                                      args=(item.name, item.config.getoption("--qtest_test_suite"), report.outcome, datetime.now(), datetime.now()),
                                      test_name=item.name)
        update_thread.start()
        pytest.threads.append(update_thread)
        return

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

    test_name = request.node.name
    test_suite_id = request.config.getoption("--qtest_test_suite")
    outcome = request.node.report.outcome

    update_thread = PropagatingThread(target=update_test_result,
                                      args=(test_name, test_suite_id, outcome, test_start_date, test_end_date),
                                      test_name=request.node.name)
    update_thread.start()
    pytest.threads.append(update_thread)

def update_test_result(test_name, test_suite_id, outcome, test_start_date, test_end_date):
    match = re.match(r"test_(\d+)", test_name)

    if not match:
        raise Exception(f"Test doesn't contain a valid ID")

    test_case = f"TC-{match.groups()[0]}"
    test_run_id = None
    test_case_id = pytest.qtest_helper.get_test_case_pid_by_id(test_case)

    if test_case_id not in pytest.tests_in_suite:
        test_run_id = pytest.qtest_helper.add_test_run_to_test_suite(test_case_id, test_suite_id)
    else:
        test_run_id = pytest.tests_in_suite[test_case_id]

    pytest.qtest_helper.update_test_run_status_in_test_suite(test_run_id, outcome, test_start_date, test_end_date)
