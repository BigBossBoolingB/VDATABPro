import unittest
import os
import glob
import shelve

from vdatabprot.rol import write
from vdatabprot.diee import run_integrity_patrol
from vdatabprot.storage.core import _DB_FILE

class TestDIEE(unittest.TestCase):
    def tearDown(self):
        """
        Clean up the database file after each test.
        """
        for f in glob.glob(f"{_DB_FILE}*"):
            os.remove(f)

    def test_integrity_patrol(self):
        """
        Tests that the integrity patrol can identify corrupted vectors.
        """
        # Create some data and write it to the database
        original_data = os.urandom(1024)
        vector_id = write(original_data)

        # Corrupt the data by modifying the compressed payload
        with shelve.open(_DB_FILE) as db:
            vector = db[vector_id]
            vector.compressed_payload = os.urandom(len(vector.compressed_payload))
            db[vector_id] = vector

        # Run the integrity patrol and check that it identifies the corruption
        corrupted_vectors = run_integrity_patrol()
        self.assertEqual(len(corrupted_vectors), 1)
        self.assertEqual(corrupted_vectors[0], vector_id)

if __name__ == '__main__':
    unittest.main()
