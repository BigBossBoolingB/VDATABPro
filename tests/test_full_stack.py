import unittest
import os
import glob
import time

from vdatabprot.rol import write, read, _prefetch_cache
from vdatabprot.diee import analyze_access_patterns

class TestFullStack(unittest.TestCase):
    def tearDown(self):
        """
        Clean up the database and log files after each test.
        """
        for f in glob.glob("vdatabprot.db*"):
            os.remove(f)
        for f in glob.glob("vdatabprot_links.db*"):
            os.remove(f)
        if os.path.exists("vdatabprot_access.log"):
            os.remove("vdatabprot_access.log")
        _prefetch_cache.clear()

    def test_prefetching(self):
        """
        Tests that the full stack can identify access patterns and pre-fetch vectors.
        """
        # Write three vectors in quick succession
        id_a = write(os.urandom(10))
        time.sleep(0.1)
        id_b = write(os.urandom(10))
        time.sleep(0.1)
        id_c = write(os.urandom(10))

        # Run the access pattern analysis
        analyze_access_patterns()

        # Clear the cache and read vector A
        _prefetch_cache.clear()
        read(id_a)

        # Check that vector B is now in the cache
        self.assertIn(id_b, _prefetch_cache)

if __name__ == '__main__':
    unittest.main()
