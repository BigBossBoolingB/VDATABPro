import unittest
import os
import glob

from vdatabprot.rol import write, read

class TestROL(unittest.TestCase):
    def tearDown(self):
        """
        Clean up the database file after each test.
        """
        for f in glob.glob("vdatabprot.db*"):
            os.remove(f)

    def test_write_read(self):
        """
        Tests that data can be written and read back correctly.
        """
        original_data = os.urandom(1024)
        vector_id = write(original_data)
        retrieved_data = read(vector_id)
        self.assertEqual(original_data, retrieved_data)

if __name__ == '__main__':
    unittest.main()
