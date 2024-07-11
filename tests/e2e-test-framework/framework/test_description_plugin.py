import inspect
import pytest


class TestDescriptionPlugin:

    def __init__(self, terminal_reporter: str):
        self.terminal_reporter = terminal_reporter
        self.desc = None

    def pytest_runtest_protocol(self, item):
        self.desc = inspect.getdoc(item.obj)

    @pytest.hookimpl(hookwrapper=True, tryfirst=True)
    def pytest_runtest_logstart(self) -> None:
        if self.terminal_reporter.verbosity == 0:
            yield
        else:
            self.terminal_reporter.write('\n\n')
            yield
            if self.desc:
                self.terminal_reporter.write(f'\n{self.desc} \n')
