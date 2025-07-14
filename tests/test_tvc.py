import unittest
import os

from vdatabprot.tvc import create_data_state_vector, reconstitute_from_vector

class TestTVC(unittest.TestCase):
    def test_reconstitution(self):
        """
        Tests that the reconstitution process is lossless.
        """
        original_data = os.urandom(1024)
        vector = create_data_state_vector(original_data)
        reconstituted_data = reconstitute_from_vector(vector)
        self.assertEqual(original_data, reconstituted_data)

if __name__ == '__main__':
    unittest.main()
