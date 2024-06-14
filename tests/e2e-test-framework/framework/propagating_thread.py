from threading import Thread


class PropagatingThread(Thread):
    def __init__(self, *args, **kwargs):
        self.target = kwargs.get('target')
        self.test_name = kwargs.get('test_name', None)

        if self.test_name is None:
            raise ValueError("Missing kwargs test_name")

        else:
            kwargs.pop('test_name')
        self.args = kwargs.get('args', ())
        self.kwargs = kwargs.get('kwargs', {})
        super(PropagatingThread, self).__init__(*args, **kwargs)
        self.exc = None

    def run(self):
        try:
            if self.target:
                self.target(*self.args, **self.kwargs)
        except Exception as exc:
            self.exc = exc

    def has_failed(self):
        return self.exc is not None

    def get_target_name(self):
        return self.target.__name__

    def join(self, timeout=None):
        super(PropagatingThread, self).join(timeout)
        if self.exc is not None:
            raise RuntimeError(f"{self.test_name} {self.exc}")
