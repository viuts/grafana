import { OracleDatasource } from './datasource';
import { OracleQueryCtrl } from './query_ctrl';

class OracleConfigCtrl {
  static templateUrl = 'partials/config.html';

  current: any;

  /** @ngInject **/
  constructor($scope) {
    this.current.jsonData.sslmode = this.current.jsonData.sslmode || 'verify-full';
  }
}

const defaultQuery = `SELECT
  extract(epoch from time_column) AS time,
  text_column as text,
  tags_column as tags
FROM
  metric_table
WHERE
  $__timeFilter(time_column)
`;

class OracleAnnotationsQueryCtrl {
  static templateUrl = 'partials/annotations.editor.html';

  annotation: any;

  /** @ngInject **/
  constructor() {
    this.annotation.rawQuery = this.annotation.rawQuery || defaultQuery;
  }
}

export {
  OracleDatasource,
  OracleDatasource as Datasource,
  OracleQueryCtrl as QueryCtrl,
  OracleConfigCtrl as ConfigCtrl,
  OracleAnnotationsQueryCtrl as AnnotationsQueryCtrl,
};
