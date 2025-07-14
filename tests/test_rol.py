import unittest
import os

from vdatabprot.rol import write, read

class TestROL(unittest.TestCase):
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
