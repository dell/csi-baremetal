# CSI baremetal E2E test framework
Test automation framwework for csi-baremetal project

## Prerequisites
```
# Install Python 3.12.2 and requirements on SUSE SP4/SP5

zypper install gcc zlib-devel libopenssl-devel libffi-devel

mkdir ~/src
mkdir ~/venv
mkdir ~/.python3.12.2

cd ~/src
wget https://www.python.org/ftp/python/3.12.2/Python-3.12.2.tgz
tar xzvf Python-3.12.2.tgz
cd Python-3.12.2/
./configure --prefix=$HOME/.python3.12.2/ --enable-optimizations
make install
export PATH=~/.python3.12.2/bin:$PATH

pip3 install virtualenv
virtualenv ~/venv/python3.12.2
. ~/venv/python3.12.2/bin/activate
python3 --version

cd <PATH TO PROJECT REPO>/tests/e2e-test-framework
pip3 install -r requirements.txt
pytest --version
```

# Update conftest.py
For local usage you can update the follwing default values:
* ssh login and password for kubernetes nodes
* namespace of atlantic installer
* qtest access token in case you want to link test case to requirements
* qtest test suite in caes you want to update test results in qtest

...or provide during cli execution:

```
pytest  --login=<login> --password=<password> --namespace=<namespace> --qtest_token=<qtest_access_token> --qtest_test_suite=<test_suite_id>
```

## Test execution
Single test case:

```
pytest -k test_1000_example
pytest -m example
```

All test cases:

```
pytest
```

## Test execution reporting

Test ecxecution report is available ```report/pytest_html_report.html```

## Before code merge
Static code analysis should by done by execution ```tox``` command

 **Note:** ```rm -rf .tox``` command will clear existing tox. 
